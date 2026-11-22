package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"

	"wsi/internal/banco"
	"wsi/internal/pdf"
)

// pdfRecebido é um upload já validado, salvo em diretório temporário.
type pdfRecebido struct {
	Caminho      string
	NomeOriginal string
	limpar       func()
}

func (p *pdfRecebido) Limpar() {
	if p != nil && p.limpar != nil {
		p.limpar()
	}
}

// receberPDF lê o campo multipart "arquivo", limita o tamanho e valida tipo MIME,
// assinatura real do arquivo (%PDF-) e estrutura (qpdf). A extensão não é considerada.
func (s *Servidor) receberPDF(w http.ResponseWriter, r *http.Request) (*pdfRecebido, error) {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.TamanhoMaximoUpload+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var maxBytes *http.MaxBytesError
		if errors.As(err, &maxBytes) {
			return nil, err
		}
		return nil, erroCampo("arquivo", "Envie um arquivo PDF.")
	}
	arquivo, cabecalho, err := r.FormFile("arquivo")
	if err != nil {
		return nil, erroCampo("arquivo", "Selecione um arquivo PDF.")
	}
	defer arquivo.Close()
	if cabecalho.Size > s.cfg.TamanhoMaximoUpload {
		return nil, erroCampo("arquivo", fmt.Sprintf("O arquivo excede o limite de %d MB.", s.cfg.TamanhoMaximoUpload/1024/1024))
	}
	if cabecalho.Size == 0 {
		return nil, erroCampo("arquivo", "O arquivo está vazio.")
	}
	dir, err := s.arm.DiretorioTemporario("upload")
	if err != nil {
		return nil, err
	}
	rec := &pdfRecebido{Caminho: filepath.Join(dir, "arquivo.pdf"), NomeOriginal: nomeArquivoSeguro(cabecalho.Filename),
		limpar: func() { os.RemoveAll(dir) }}
	destino, err := os.Create(rec.Caminho)
	if err != nil {
		rec.Limpar()
		return nil, err
	}
	_, err = io.Copy(destino, arquivo)
	destino.Close()
	if err != nil {
		rec.Limpar()
		return nil, err
	}
	f, err := os.Open(rec.Caminho)
	if err != nil {
		rec.Limpar()
		return nil, err
	}
	inicio := make([]byte, 512)
	n, _ := io.ReadFull(f, inicio)
	f.Close()
	if mime := http.DetectContentType(inicio[:n]); mime != "application/pdf" {
		rec.Limpar()
		return nil, erroCampo("arquivo", "O arquivo enviado não é um PDF.")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	if err := pdf.Validar(ctx, rec.Caminho); err != nil {
		rec.Limpar()
		return nil, erroCampo("arquivo", "PDF inválido ou corrompido: "+err.Error())
	}
	return rec, nil
}

func nomeArquivoSeguro(nome string) string {
	nome = filepath.Base(strings.ReplaceAll(nome, "\\", "/"))
	nome = strings.Map(func(r rune) rune {
		if r < 32 || r == '"' || r == '/' {
			return -1
		}
		return r
	}, nome)
	if len(nome) > 200 {
		nome = nome[:200]
	}
	if nome == "" || nome == "." {
		nome = "arquivo.pdf"
	}
	return nome
}

// nomeDownload gera nomes ASCII previsíveis: "ATA_TCC_Fulano_de_Tal.pdf".
func nomeDownload(partes ...string) string {
	base := norm.NFD.String(strings.Join(partes, "_"))
	var b strings.Builder
	for _, r := range base {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('_')
		}
	}
	s := b.String()
	if len(s) > 120 {
		s = s[:120]
	}
	return s + ".pdf"
}

// enviarArquivo transmite um arquivo privado após a autorização (com suporte a Range).
func (s *Servidor) enviarArquivo(w http.ResponseWriter, r *http.Request, chave, nome, mime string, inline bool) error {
	f, err := s.arm.Abrir(chave)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return erroNaoEncontrado("Arquivo não encontrado no armazenamento.")
		}
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	disposicao := "attachment"
	if inline {
		disposicao = "inline"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, disposicao, nome, url.PathEscape(nome)))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, nome, info.ModTime(), f)
	return nil
}

// ---------------------------------------------------------------------
// Autorização por objeto
// ---------------------------------------------------------------------

type acessoTrabalho struct {
	AlunoID        string
	OrientadorID   string
	EventoID       string
	EhAluno        bool
	EhOrientador   bool
	EhBanca        bool // membro ativo da banca (inclui o orientador)
	EhApresentador bool
}

func (a acessoTrabalho) algum() bool { return a.EhAluno || a.EhOrientador || a.EhBanca || a.EhApresentador }

// acessoAoTrabalho calcula o vínculo do usuário da sessão com o trabalho, conforme o contexto ativo.
// ADMIN não recebe acesso automático ao conteúdo dos trabalhos.
func acessoAoTrabalho(ctx context.Context, q banco.Executor, ses *Sessao, trabalhoID string) (acessoTrabalho, error) {
	var a acessoTrabalho
	var orientador, evento *string
	err := q.QueryRow(ctx, `SELECT aluno_id::text, orientador_id::text, evento_id::text FROM trabalhos WHERE id = $1`, trabalhoID).
		Scan(&a.AlunoID, &orientador, &evento)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, erroNaoEncontrado("Trabalho não encontrado.")
	}
	if err != nil {
		return a, err
	}
	if orientador != nil {
		a.OrientadorID = *orientador
	}
	if evento != nil {
		a.EventoID = *evento
	}
	switch ses.Contexto {
	case PerfilAluno:
		a.EhAluno = a.AlunoID == ses.UsuarioID
	case PerfilDocente:
		a.EhOrientador = a.OrientadorID == ses.UsuarioID
		err = q.QueryRow(ctx, `SELECT
			EXISTS(SELECT 1 FROM membros_banca WHERE trabalho_id = $1 AND docente_id = $2 AND situacao = 'ativo'),
			EXISTS(SELECT 1 FROM apresentadores_evento ap JOIN trabalhos t ON t.evento_id = ap.evento_id
			       WHERE t.id = $1 AND ap.docente_id = $2)`, trabalhoID, ses.UsuarioID).Scan(&a.EhBanca, &a.EhApresentador)
	}
	return a, err
}

// GET /arquivos/{id} — artigo ou apresentação: aluno autor, orientador, banca ou apresentador do evento.
func (s *Servidor) baixarArquivoTrabalho(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	var trabalhoID, tipo, chave, nomeOriginal, aluno string
	var versao int
	err := s.pool.QueryRow(r.Context(), `SELECT af.trabalho_id::text, af.tipo, af.chave_armazenamento, af.nome_original, af.versao, u.nome_completo
		FROM arquivos_trabalho af JOIN trabalhos t ON t.id = af.trabalho_id JOIN usuarios u ON u.id = t.aluno_id WHERE af.id = $1`,
		chi.URLParam(r, "id")).Scan(&trabalhoID, &tipo, &chave, &nomeOriginal, &versao, &aluno)
	if err != nil {
		return erroNaoEncontrado("Arquivo não encontrado.")
	}
	acesso, err := acessoAoTrabalho(r.Context(), s.pool, ses, trabalhoID)
	if err != nil {
		return err
	}
	if !acesso.algum() {
		return erroNaoEncontrado("Arquivo não encontrado.")
	}
	rotulo := "Artigo"
	if tipo == "apresentacao" {
		rotulo = "Apresentacao"
	}
	return s.enviarArquivo(w, r, chave, nomeDownload(rotulo, aluno, fmt.Sprintf("v%d", versao)), "application/pdf", r.URL.Query().Get("visualizar") == "1")
}

// GET /documentos/{id} — declaração gerada: o próprio aluno ou o DOCENTE orientador ativo.
func (s *Servidor) baixarDocumentoGerado(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	var entidade, chave, nome string
	var orientadorAtivo bool
	err := s.pool.QueryRow(r.Context(), `SELECT d.entidade_id::text, d.chave_armazenamento, u.nome_completo,
		EXISTS(SELECT 1 FROM vinculos_orientacao v WHERE v.aluno_id = d.entidade_id AND v.docente_id::text = $2 AND v.situacao = 'ativo')
		FROM documentos_gerados d JOIN usuarios u ON u.id = d.entidade_id WHERE d.id = $1 AND d.tipo = 'declaracao_antiplagio'`,
		chi.URLParam(r, "id"), ses.UsuarioID).Scan(&entidade, &chave, &nome, &orientadorAtivo)
	permitido := (ses.Contexto == PerfilAluno && entidade == ses.UsuarioID) || (ses.Contexto == PerfilDocente && orientadorAtivo)
	if err != nil || !permitido {
		return erroNaoEncontrado("Documento não encontrado.")
	}
	return s.enviarArquivo(w, r, chave, nomeDownload("Declaracao_Anti_Plagio", nome), "application/pdf", false)
}

// GET /declaracoes-assinadas/{id} — o próprio aluno ou o DOCENTE orientador ativo.
func (s *Servidor) baixarDeclaracaoAssinada(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	var alunoID, chave, nome string
	var orientadorAtivo bool
	err := s.pool.QueryRow(r.Context(), `SELECT d.aluno_id::text, d.chave_armazenamento, u.nome_completo,
		EXISTS(SELECT 1 FROM vinculos_orientacao v WHERE v.aluno_id = d.aluno_id AND v.docente_id::text = $2 AND v.situacao = 'ativo')
		FROM declaracoes_assinadas d JOIN usuarios u ON u.id = d.aluno_id WHERE d.id = $1`, chi.URLParam(r, "id"), ses.UsuarioID).
		Scan(&alunoID, &chave, &nome, &orientadorAtivo)
	if err != nil {
		return erroNaoEncontrado("Documento não encontrado.")
	}
	permitido := (ses.Contexto == PerfilAluno && alunoID == ses.UsuarioID) || (ses.Contexto == PerfilDocente && orientadorAtivo)
	if !permitido {
		return erroNaoEncontrado("Documento não encontrado.")
	}
	return s.enviarArquivo(w, r, chave, nomeDownload("Declaracao_Anti_Plagio_Assinada", nome), "application/pdf", false)
}

// ---------------------------------------------------------------------
// Estado das tarefas da fila
// ---------------------------------------------------------------------

type tarefaResposta struct {
	ID          string  `json:"id"`
	Tipo        string  `json:"tipo"`
	Estado      string  `json:"estado"`
	Progresso   int     `json:"progresso"`
	Erro        string  `json:"erro"`
	Avisos      string  `json:"avisos"`
	ResultadoID *string `json:"resultadoId"`
	CriadoEm    string  `json:"criadoEm"`
}

func (s *Servidor) obterTarefa(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	var t tarefaResposta
	var solicitante string
	err := s.pool.QueryRow(r.Context(), `SELECT id::text, tipo, estado, progresso, erro, avisos, resultado_id::text,
		to_char(criado_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI'), solicitado_por::text
		FROM tarefas_documentos WHERE id = $1`, chi.URLParam(r, "id")).
		Scan(&t.ID, &t.Tipo, &t.Estado, &t.Progresso, &t.Erro, &t.Avisos, &t.ResultadoID, &t.CriadoEm, &solicitante)
	if err != nil {
		return erroNaoEncontrado("Tarefa não encontrada.")
	}
	if solicitante != ses.UsuarioID && ses.Contexto != PerfilAdmin {
		// Outros participantes acompanham pelo estado do próprio objeto (ATA, publicação).
		return erroNaoEncontrado("Tarefa não encontrada.")
	}
	if t.Estado == "falhou" && ses.Contexto != PerfilAdmin {
		t.Erro = "Não foi possível gerar o documento: " + t.Erro
	}
	return responderOK(w, t)
}
