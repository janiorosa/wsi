package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"wsi/internal/fila"
	"wsi/internal/regras"
)

type orientadorResumo struct {
	VinculoID string `json:"vinculoId"`
	ID        string `json:"id"`
	Nome      string `json:"nome"`
	Email     string `json:"email"`
	Desde     string `json:"desde"`
}

type trabalhoAluno struct {
	ID           string                       `json:"id"`
	Titulo       string                       `json:"titulo"`
	Resumo       string                       `json:"resumo"`
	LinkOverleaf string                       `json:"linkOverleaf"`
	Estado       string                       `json:"estado"`
	SubmetidoEm  *string                      `json:"submetidoEm"`
	Evento       *eventoTrabalho              `json:"evento"`
	Arquivos     map[string][]arquivoTrabalho `json:"arquivos"`
	Editavel     bool                         `json:"editavel"`
}

func (s *Servidor) alunoPainel(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	ctx := r.Context()
	resp := map[string]any{}

	var nome, email, matricula, curso string
	if err := s.pool.QueryRow(ctx, `SELECT u.nome_completo, u.email, COALESCE(pa.matricula, ''), COALESCE(pa.curso, '')
		FROM usuarios u LEFT JOIN perfis_aluno pa ON pa.usuario_id = u.id WHERE u.id = $1`, ses.UsuarioID).
		Scan(&nome, &email, &matricula, &curso); err != nil {
		return err
	}
	resp["aluno"] = map[string]string{"nome": nome, "email": email, "matricula": matricula, "curso": curso}

	var orientador *orientadorResumo
	var o orientadorResumo
	err := s.pool.QueryRow(ctx, `SELECT v.id::text, d.id::text, d.nome_completo, d.email, to_char(v.inicio AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY')
		FROM vinculos_orientacao v JOIN usuarios d ON d.id = v.docente_id WHERE v.aluno_id = $1 AND v.situacao = 'ativo'`, ses.UsuarioID).
		Scan(&o.VinculoID, &o.ID, &o.Nome, &o.Email, &o.Desde)
	if err == nil {
		orientador = &o
	}
	resp["orientador"] = orientador

	trabalhoID, err := trabalhoDoAluno(ctx, s.pool, ses.UsuarioID)
	if err != nil {
		return err
	}
	var trabalho *trabalhoAluno
	var ata *ataResumo
	if trabalhoID != "" {
		var t trabalhoAluno
		if err := s.pool.QueryRow(ctx, `SELECT id::text, titulo, resumo, link_overleaf, estado,
			to_char(submetido_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI') FROM trabalhos WHERE id = $1`, trabalhoID).
			Scan(&t.ID, &t.Titulo, &t.Resumo, &t.LinkOverleaf, &t.Estado, &t.SubmetidoEm); err != nil {
			return err
		}
		if t.Evento, err = eventoDoTrabalho(ctx, s.pool, trabalhoID); err != nil {
			return err
		}
		if t.Arquivos, err = listarArquivos(ctx, s.pool, trabalhoID); err != nil {
			return err
		}
		if ata, err = resumoATA(ctx, s.pool, trabalhoID, ses.UsuarioID); err != nil {
			return err
		}
		t.Editavel = ata == nil
		trabalho = &t
	}
	resp["trabalho"] = trabalho
	resp["ata"] = ata

	declaracao, err := resumoDeclaracao(ctx, s.pool, ses.UsuarioID, true)
	if err != nil {
		return err
	}
	resp["declaracao"] = declaracao
	resp["proximosPassos"] = proximosPassosAluno(orientador, trabalho, ata, declaracao, matricula)
	return responderOK(w, resp)
}

func proximosPassosAluno(o *orientadorResumo, t *trabalhoAluno, ata *ataResumo, d declaracaoResumo, matricula string) []string {
	var passos []string
	if matricula == "" {
		passos = append(passos, "Complete sua matrícula em “Minha conta”.")
	}
	if o == nil {
		passos = append(passos, "Escolha seu orientador.")
	}
	if t == nil || vazio(t.Titulo) || vazio(t.Resumo) {
		passos = append(passos, "Cadastre o título e o resumo do seu TCC.")
	}
	if t == nil || len(t.Arquivos["artigo"]) == 0 {
		passos = append(passos, "Envie o PDF do artigo.")
	}
	if t == nil || len(t.Arquivos["apresentacao"]) == 0 {
		passos = append(passos, "Envie o PDF da apresentação.")
	}
	if t != nil && t.Evento == nil && o != nil {
		passos = append(passos, "Aguarde seu orientador associar o trabalho a um evento.")
	}
	if d.AssinadaID == nil {
		if d.GeradaID == nil {
			passos = append(passos, "Gere a declaração anti-plágio.")
		}
		passos = append(passos, "Assine a declaração anti-plágio e envie o PDF assinado.")
	}
	if ata != nil && ata.MinhaVez {
		passos = append(passos, "Baixe a ATA, assine e envie o PDF assinado (você é o primeiro a assinar).")
	}
	if len(passos) == 0 {
		if ata != nil && ata.Estado == "finalizada" {
			passos = append(passos, "Tudo concluído! A ATA final está disponível para download.")
		} else {
			passos = append(passos, "Nada pendente no momento. Aguarde a avaliação da banca.")
		}
	}
	return passos
}

func validarLinkOverleaf(link string) bool {
	if link == "" {
		return true
	}
	u, err := url.Parse(link)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

// PUT /aluno/trabalho {titulo, resumo, linkOverleaf}
func (s *Servidor) alunoSalvarTrabalho(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	var req struct {
		Titulo       string `json:"titulo"`
		Resumo       string `json:"resumo"`
		LinkOverleaf string `json:"linkOverleaf"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	req.Titulo = strings.Join(strings.Fields(req.Titulo), " ")
	req.Resumo = strings.TrimSpace(req.Resumo)
	req.LinkOverleaf = strings.TrimSpace(req.LinkOverleaf)
	campos := map[string]string{}
	if utf8.RuneCountInString(req.Titulo) > 400 {
		campos["titulo"] = "O título deve ter no máximo 400 caracteres."
	}
	if utf8.RuneCountInString(req.Resumo) > 6000 {
		campos["resumo"] = "O resumo deve ter no máximo 6000 caracteres."
	}
	if !validarLinkOverleaf(req.LinkOverleaf) {
		campos["linkOverleaf"] = "Informe um endereço válido (https://...)."
	}
	if len(campos) > 0 {
		return erroValidacao("Verifique os campos destacados.", campos)
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	id, err := garantirTrabalho(r.Context(), tx, ses.UsuarioID, ses.UsuarioID)
	if err != nil {
		return err
	}
	var possuiATA bool
	if err := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM atas WHERE trabalho_id = $1 AND estado <> 'cancelada')`, id).Scan(&possuiATA); err != nil {
		return err
	}
	if possuiATA {
		return erroConflito("O título e o resumo não podem mais ser alterados: a ATA já foi gerada.")
	}
	if _, err := tx.Exec(r.Context(), `UPDATE trabalhos SET titulo = $2, resumo = $3, link_overleaf = $4, atualizado_em = now() WHERE id = $1`,
		id, req.Titulo, req.Resumo, req.LinkOverleaf); err != nil {
		return err
	}
	if _, err := regras.RecalcularEstadoTrabalho(r.Context(), tx, id, ses.UsuarioID, "Dados do trabalho atualizados"); err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "salvar_trabalho", "trabalho", id, nil); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	return responderOK(w, map[string]string{"id": id})
}

// POST /aluno/trabalho/arquivos/{tipo} — artigo ou apresentação, em momentos independentes; versões preservadas.
func (s *Servidor) alunoEnviarArquivo(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	tipo := chi.URLParam(r, "tipo")
	if tipo != "artigo" && tipo != "apresentacao" {
		return erroNaoEncontrado("")
	}
	rec, err := s.receberPDF(w, r)
	if err != nil {
		return err
	}
	defer rec.Limpar()
	categoria := map[string]string{"artigo": "artigos", "apresentacao": "apresentacoes"}[tipo]
	salvo, err := s.arm.SalvarDeArquivo(categoria, rec.Caminho)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		s.arm.Remover(salvo.Chave)
		return err
	}
	defer tx.Rollback(context.Background())
	var arquivoID string
	var versao int
	err = func() error {
		trabalhoID, err := garantirTrabalho(r.Context(), tx, ses.UsuarioID, ses.UsuarioID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `SELECT id FROM trabalhos WHERE id = $1 FOR UPDATE`, trabalhoID); err != nil {
			return err
		}
		if err := tx.QueryRow(r.Context(), `INSERT INTO arquivos_trabalho (trabalho_id, tipo, nome_original, chave_armazenamento, mime, tamanho, sha256, versao, enviado_por)
			VALUES ($1, $2, $3, $4, 'application/pdf', $5, $6,
			        (SELECT COALESCE(max(versao), 0) + 1 FROM arquivos_trabalho WHERE trabalho_id = $1 AND tipo = $2), $7)
			RETURNING id::text, versao`, trabalhoID, tipo, rec.NomeOriginal, salvo.Chave, salvo.Tamanho, salvo.SHA256, ses.UsuarioID).
			Scan(&arquivoID, &versao); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `UPDATE trabalhos SET atualizado_em = now() WHERE id = $1`, trabalhoID); err != nil {
			return err
		}
		if err := s.auditar(r.Context(), tx, r, "enviar_"+tipo, "trabalho", trabalhoID,
			map[string]any{"arquivo": arquivoID, "versao": versao, "sha256": salvo.SHA256, "tamanho": salvo.Tamanho}); err != nil {
			return err
		}
		return tx.Commit(r.Context())
	}()
	if err != nil {
		s.arm.Remover(salvo.Chave)
		return err
	}
	responderJSON(w, http.StatusCreated, map[string]any{"id": arquivoID, "versao": versao, "sha256": salvo.SHA256})
	return nil
}

// POST /aluno/orientador {docenteId, confirmarTroca}
func (s *Servidor) alunoEscolherOrientador(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	var req struct {
		DocenteID      string `json:"docenteId"`
		ConfirmarTroca bool   `json:"confirmarTroca"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	id, err := criarVinculo(r.Context(), tx, ses.UsuarioID, req.DocenteID, ses.UsuarioID, req.ConfirmarTroca)
	if err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "aluno_escolher_orientador", "vinculo_orientacao", id,
		map[string]any{"docente": req.DocenteID, "troca": req.ConfirmarTroca}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	responderJSON(w, http.StatusCreated, map[string]string{"id": id})
	return nil
}

// POST /aluno/declaracao/gerar — coloca a geração na fila e devolve a tarefa para acompanhamento.
func (s *Servidor) alunoGerarDeclaracao(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	var matricula string
	if err := s.pool.QueryRow(r.Context(), `SELECT COALESCE(matricula, '') FROM perfis_aluno WHERE usuario_id = $1`, ses.UsuarioID).Scan(&matricula); err != nil {
		return err
	}
	if vazio(matricula) {
		return erroRequisicao("Informe sua matrícula em “Minha conta” antes de gerar a declaração.")
	}
	tarefa, err := fila.Enfileirar(r.Context(), s.pool, fila.TipoDeclaracao, ses.UsuarioID, nil, ses.UsuarioID)
	if err != nil {
		return err
	}
	s.fila.Notificar()
	s.auditar(r.Context(), s.pool, r, "solicitar_declaracao", "aluno", ses.UsuarioID, map[string]any{"tarefa": tarefa})
	responderJSON(w, http.StatusAccepted, map[string]string{"tarefaId": tarefa})
	return nil
}

// POST /aluno/declaracao/assinada — PDF assinado externamente.
func (s *Servidor) alunoEnviarDeclaracaoAssinada(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	rec, err := s.receberPDF(w, r)
	if err != nil {
		return err
	}
	defer rec.Limpar()
	salvo, err := s.arm.SalvarDeArquivo("declaracoes-assinadas", rec.Caminho)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		s.arm.Remover(salvo.Chave)
		return err
	}
	defer tx.Rollback(context.Background())
	var id string
	err = func() error {
		if _, err := tx.Exec(r.Context(), `SELECT id FROM usuarios WHERE id = $1 FOR UPDATE`, ses.UsuarioID); err != nil {
			return err
		}
		if err := tx.QueryRow(r.Context(), `INSERT INTO declaracoes_assinadas (aluno_id, nome_original, chave_armazenamento, tamanho, sha256, versao, enviado_por)
			VALUES ($1, $2, $3, $4, $5, (SELECT COALESCE(max(versao), 0) + 1 FROM declaracoes_assinadas WHERE aluno_id = $1), $1)
			RETURNING id::text`, ses.UsuarioID, rec.NomeOriginal, salvo.Chave, salvo.Tamanho, salvo.SHA256).Scan(&id); err != nil {
			return err
		}
		if err := regras.RecalcularTrabalhosDoAluno(r.Context(), tx, ses.UsuarioID, ses.UsuarioID, "Declaração anti-plágio assinada enviada"); err != nil {
			return err
		}
		if err := s.auditar(r.Context(), tx, r, "enviar_declaracao_assinada", "aluno", ses.UsuarioID,
			map[string]any{"documento": id, "sha256": salvo.SHA256}); err != nil {
			return err
		}
		return tx.Commit(r.Context())
	}()
	if err != nil {
		s.arm.Remover(salvo.Chave)
		return err
	}
	responderJSON(w, http.StatusCreated, map[string]string{"id": id})
	return nil
}
