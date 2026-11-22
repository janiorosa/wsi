package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"

	"wsi/internal/banco"
	"wsi/internal/premiacoes"
	"wsi/internal/regras"
)

var estadosEvento = map[string]string{
	"rascunho":      "Rascunho",
	"aberto":        "Aberto para submissões",
	"em_preparacao": "Em preparação",
	"em_andamento":  "Em andamento",
	"encerrado":     "Encerrado",
	"publicado":     "Publicado",
}

type evento struct {
	ID                  string  `json:"id"`
	Nome                string  `json:"nome"`
	Edicao              string  `json:"edicao"`
	NumeroEdicao        *int    `json:"numeroEdicao"`
	Descricao           string  `json:"descricao"`
	DataApresentacao    string  `json:"dataApresentacao"`
	HorarioApresentacao string  `json:"horarioApresentacao"`
	Local               string  `json:"local"`
	Campus              string  `json:"campus"`
	Estado              string  `json:"estado"`
	PrazoSubmissao      *string `json:"prazoSubmissao"`
	PrazoApresentacao   *string `json:"prazoApresentacao"`
	InformacoesPublicas string  `json:"informacoesPublicas"`
	Slug                string  `json:"slug"`
	TemCapa             bool    `json:"temCapa"`
	Futuro              bool    `json:"futuro"`
	TotalTrabalhos      int     `json:"totalTrabalhos"`
}

const selecaoEvento = `SELECT e.id::text, e.nome, e.edicao, e.numero_edicao, e.descricao, to_char(e.data_apresentacao, 'YYYY-MM-DD'),
	to_char(e.horario_apresentacao, 'HH24:MI'), e.local, e.campus, e.estado,
	to_char(e.prazo_submissao, 'YYYY-MM-DD'), to_char(e.prazo_apresentacao, 'YYYY-MM-DD'), e.informacoes_publicas, e.slug,
	e.capa_chave IS NOT NULL,
	(e.data_apresentacao + e.horario_apresentacao) > (now() AT TIME ZONE 'America/Sao_Paulo'),
	(SELECT count(*) FROM trabalhos t WHERE t.evento_id = e.id AND t.estado <> 'cancelado')
	FROM eventos e`

func escanearEvento(row interface{ Scan(...any) error }) (evento, error) {
	var e evento
	err := row.Scan(&e.ID, &e.Nome, &e.Edicao, &e.NumeroEdicao, &e.Descricao, &e.DataApresentacao, &e.HorarioApresentacao, &e.Local, &e.Campus, &e.Estado,
		&e.PrazoSubmissao, &e.PrazoApresentacao, &e.InformacoesPublicas, &e.Slug, &e.TemCapa, &e.Futuro, &e.TotalTrabalhos)
	return e, err
}

func listarEventos(ctx context.Context, q banco.Executor, filtro string, args ...any) ([]evento, error) {
	rows, err := q.Query(ctx, selecaoEvento+" "+filtro, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	itens := []evento{}
	for rows.Next() {
		e, err := escanearEvento(rows)
		if err != nil {
			return nil, err
		}
		itens = append(itens, e)
	}
	return itens, rows.Err()
}

type entradaEvento struct {
	Nome                string `json:"nome"`
	Edicao              string `json:"edicao"`
	NumeroEdicao        *int   `json:"numeroEdicao"`
	Descricao           string `json:"descricao"`
	DataApresentacao    string `json:"dataApresentacao"`
	HorarioApresentacao string `json:"horarioApresentacao"`
	Local               string `json:"local"`
	Campus              string `json:"campus"`
	Estado              string `json:"estado"`
	PrazoSubmissao      string `json:"prazoSubmissao"`
	PrazoApresentacao   string `json:"prazoApresentacao"`
	InformacoesPublicas string `json:"informacoesPublicas"`
}

var (
	regexData    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	regexHorario = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
)

func dataValida(s string) bool {
	if !regexData.MatchString(s) {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func (e *entradaEvento) validar() error {
	e.Nome = strings.TrimSpace(e.Nome)
	e.Local = strings.TrimSpace(e.Local)
	e.Edicao = strings.TrimSpace(e.Edicao)
	if e.Campus = strings.TrimSpace(e.Campus); e.Campus == "" {
		e.Campus = "Ouro Branco"
	}
	if e.Estado == "" {
		e.Estado = "rascunho"
	}
	campos := map[string]string{}
	if e.Nome == "" {
		campos["nome"] = "Informe o nome do evento."
	}
	if !dataValida(e.DataApresentacao) {
		campos["dataApresentacao"] = "Informe a data da apresentação."
	}
	if !regexHorario.MatchString(e.HorarioApresentacao) {
		campos["horarioApresentacao"] = "Informe o horário no formato HH:MM."
	}
	if e.Local == "" {
		campos["local"] = "Informe o local."
	}
	if _, ok := estadosEvento[e.Estado]; !ok {
		campos["estado"] = "Estado inválido."
	}
	if e.NumeroEdicao != nil && (*e.NumeroEdicao < 1 || *e.NumeroEdicao > 3999) {
		campos["numeroEdicao"] = "Informe um número de edição entre 1 e 3999."
	}
	if e.PrazoSubmissao != "" && !dataValida(e.PrazoSubmissao) {
		campos["prazoSubmissao"] = "Data inválida."
	}
	if e.PrazoApresentacao != "" && !dataValida(e.PrazoApresentacao) {
		campos["prazoApresentacao"] = "Data inválida."
	}
	if len(campos) > 0 {
		return erroValidacao("Verifique os campos destacados.", campos)
	}
	return nil
}

// gerarSlug cria o endereço público a partir do nome e da edição (sem acentos).
func gerarSlug(nome, edicao string) string {
	base := norm.NFD.String(strings.ToLower(nome + " " + edicao))
	var b strings.Builder
	hifen := false
	for _, r := range base {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			hifen = false
		default:
			if !hifen && b.Len() > 0 {
				b.WriteByte('-')
				hifen = true
			}
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > 80 {
		s = strings.Trim(s[:80], "-")
	}
	if s == "" {
		s = "evento"
	}
	return s
}

func (s *Servidor) adminListarEventos(w http.ResponseWriter, r *http.Request) error {
	estado := r.URL.Query().Get("estado")
	itens, err := listarEventos(r.Context(), s.pool, `WHERE ($1 = '' OR e.estado = $1) ORDER BY e.data_apresentacao DESC`, estado)
	if err != nil {
		return err
	}
	return responderOK(w, itens)
}

func (s *Servidor) adminCriarEvento(w http.ResponseWriter, r *http.Request) error {
	var e entradaEvento
	if err := lerJSON(r, &e); err != nil {
		return err
	}
	if err := e.validar(); err != nil {
		return err
	}
	ses := sessaoDe(r)
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	base := gerarSlug(e.Nome, e.Edicao)
	slug := base
	for i := 2; ; i++ {
		var existe bool
		if err := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM eventos WHERE slug = $1)`, slug).Scan(&existe); err != nil {
			return err
		}
		if !existe {
			break
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	var id string
	err = tx.QueryRow(r.Context(), `INSERT INTO eventos (nome, edicao, descricao, data_apresentacao, horario_apresentacao, local, campus, estado,
		prazo_submissao, prazo_apresentacao, informacoes_publicas, slug, criado_por, numero_edicao)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) RETURNING id::text`,
		e.Nome, e.Edicao, strings.TrimSpace(e.Descricao), e.DataApresentacao, e.HorarioApresentacao, e.Local, e.Campus, e.Estado,
		textoOuNil(e.PrazoSubmissao), textoOuNil(e.PrazoApresentacao), strings.TrimSpace(e.InformacoesPublicas), slug, ses.UsuarioID, e.NumeroEdicao).Scan(&id)
	if err != nil {
		return err
	}
	// Todo evento já nasce com a premiação padrão.
	if err := premiacoes.CriarPadrao(r.Context(), tx, id, &ses.UsuarioID); err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "criar_evento", "evento", id, map[string]any{"nome": e.Nome}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	responderJSON(w, http.StatusCreated, map[string]string{"id": id, "slug": slug})
	return nil
}

func (s *Servidor) adminAtualizarEvento(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	var e entradaEvento
	if err := lerJSON(r, &e); err != nil {
		return err
	}
	if err := e.validar(); err != nil {
		return err
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	antes, err := escanearEvento(tx.QueryRow(r.Context(), selecaoEvento+` WHERE e.id = $1`, id))
	if err == nil {
		_, err = tx.Exec(r.Context(), `SELECT id FROM eventos WHERE id = $1 FOR UPDATE`, id)
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(r.Context(), `UPDATE eventos SET nome = $2, edicao = $3, descricao = $4, data_apresentacao = $5, horario_apresentacao = $6,
		local = $7, campus = $8, estado = $9, prazo_submissao = $10, prazo_apresentacao = $11, informacoes_publicas = $12, numero_edicao = $13,
		atualizado_em = now()
		WHERE id = $1`, id, e.Nome, e.Edicao, strings.TrimSpace(e.Descricao), e.DataApresentacao, e.HorarioApresentacao, e.Local, e.Campus, e.Estado,
		textoOuNil(e.PrazoSubmissao), textoOuNil(e.PrazoApresentacao), strings.TrimSpace(e.InformacoesPublicas), e.NumeroEdicao); err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "atualizar_evento", "evento", id, map[string]any{
		"antes":  map[string]any{"data": antes.DataApresentacao, "horario": antes.HorarioApresentacao, "local": antes.Local, "estado": antes.Estado},
		"depois": map[string]any{"data": e.DataApresentacao, "horario": e.HorarioApresentacao, "local": e.Local, "estado": e.Estado},
	}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}

type sessaoEvento struct {
	ID      string `json:"id"`
	Titulo  string `json:"titulo"`
	Data    string `json:"data"`
	Horario string `json:"horario"`
	Local   string `json:"local"`
	Ordem   int    `json:"ordem"`
}

func listarSessoes(ctx context.Context, q banco.Executor, eventoID string) ([]sessaoEvento, error) {
	rows, err := q.Query(ctx, `SELECT id::text, titulo, to_char(data, 'YYYY-MM-DD'), to_char(horario, 'HH24:MI'), local, ordem
		FROM sessoes_evento WHERE evento_id = $1 ORDER BY ordem, data, horario`, eventoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	itens := []sessaoEvento{}
	for rows.Next() {
		var se sessaoEvento
		if err := rows.Scan(&se.ID, &se.Titulo, &se.Data, &se.Horario, &se.Local, &se.Ordem); err != nil {
			return nil, err
		}
		itens = append(itens, se)
	}
	return itens, rows.Err()
}

type trabalhoProgramacao struct {
	ID              string  `json:"id"`
	Titulo          string  `json:"titulo"`
	Aluno           string  `json:"aluno"`
	Orientador      *string `json:"orientador"`
	Estado          string  `json:"estado"`
	Ordem           *int    `json:"ordem"`
	Sessao          *string `json:"sessao"`
	TemArtigo       bool    `json:"temArtigo"`
	TemApresentacao bool    `json:"temApresentacao"`
}

func listarTrabalhosEvento(ctx context.Context, q banco.Executor, eventoID string) ([]trabalhoProgramacao, error) {
	rows, err := q.Query(ctx, `SELECT t.id::text, t.titulo, a.nome_completo, o.nome_completo, t.estado, t.ordem_programacao, se.titulo,
		EXISTS(SELECT 1 FROM arquivos_trabalho WHERE trabalho_id = t.id AND tipo = 'artigo'),
		EXISTS(SELECT 1 FROM arquivos_trabalho WHERE trabalho_id = t.id AND tipo = 'apresentacao')
		FROM trabalhos t JOIN usuarios a ON a.id = t.aluno_id LEFT JOIN usuarios o ON o.id = t.orientador_id
		LEFT JOIN sessoes_evento se ON se.id = t.sessao_id
		WHERE t.evento_id = $1 AND t.estado <> 'cancelado'
		ORDER BY t.ordem_programacao NULLS LAST, t.titulo, t.id`, eventoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	itens := []trabalhoProgramacao{}
	for rows.Next() {
		var t trabalhoProgramacao
		if err := rows.Scan(&t.ID, &t.Titulo, &t.Aluno, &t.Orientador, &t.Estado, &t.Ordem, &t.Sessao, &t.TemArtigo, &t.TemApresentacao); err != nil {
			return nil, err
		}
		itens = append(itens, t)
	}
	return itens, rows.Err()
}

type apresentador struct {
	DocenteID    string `json:"docenteId"`
	Nome         string `json:"nome"`
	PodePublicar bool   `json:"podePublicar"`
}

func (s *Servidor) adminObterEvento(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	e, err := escanearEvento(s.pool.QueryRow(r.Context(), selecaoEvento+` WHERE e.id = $1`, id))
	if err != nil {
		return err
	}
	sessoes, err := listarSessoes(r.Context(), s.pool, id)
	if err != nil {
		return err
	}
	trabalhos, err := listarTrabalhosEvento(r.Context(), s.pool, id)
	if err != nil {
		return err
	}
	rows, err := s.pool.Query(r.Context(), `SELECT ap.docente_id::text, u.nome_completo, ap.pode_publicar FROM apresentadores_evento ap
		JOIN usuarios u ON u.id = ap.docente_id WHERE ap.evento_id = $1 ORDER BY u.nome_completo`, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	apresentadores := []apresentador{}
	for rows.Next() {
		var a apresentador
		if err := rows.Scan(&a.DocenteID, &a.Nome, &a.PodePublicar); err != nil {
			return err
		}
		apresentadores = append(apresentadores, a)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	publicacoes, err := listarPublicacoes(r.Context(), s.pool, id, false)
	if err != nil {
		return err
	}
	resultados, err := premiacoes.Resultados(r.Context(), s.pool, id)
	if err != nil {
		return err
	}
	return responderOK(w, map[string]any{"evento": e, "sessoes": sessoes, "trabalhos": trabalhos,
		"apresentadores": apresentadores, "publicacoes": publicacoes, "premiacoes": resultados})
}

func (s *Servidor) adminEnviarCapa(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	arquivo, _, err := r.FormFile("arquivo")
	if err != nil {
		return erroCampo("arquivo", "Envie uma imagem PNG ou JPEG de até 5 MB.")
	}
	defer arquivo.Close()
	cabecalho := make([]byte, 512)
	n, _ := io.ReadFull(arquivo, cabecalho)
	mime := http.DetectContentType(cabecalho[:n])
	if mime != "image/png" && mime != "image/jpeg" {
		return erroCampo("arquivo", "A capa deve ser uma imagem PNG ou JPEG.")
	}
	salvo, err := s.arm.Salvar("capas", io.MultiReader(strings.NewReader(string(cabecalho[:n])), arquivo))
	if err != nil {
		return err
	}
	var anterior *string
	if err := s.pool.QueryRow(r.Context(), `UPDATE eventos e SET capa_chave = $2, capa_mime = $3, atualizado_em = now()
		FROM (SELECT capa_chave FROM eventos WHERE id = $1) antigo WHERE e.id = $1 RETURNING antigo.capa_chave`, id, salvo.Chave, mime).Scan(&anterior); err != nil {
		s.arm.Remover(salvo.Chave)
		return err
	}
	if anterior != nil {
		s.arm.Remover(*anterior)
	}
	s.auditar(r.Context(), s.pool, r, "enviar_capa_evento", "evento", id, nil)
	return responderOK(w, map[string]bool{"ok": true})
}

type entradaSessao struct {
	Titulo  string `json:"titulo"`
	Data    string `json:"data"`
	Horario string `json:"horario"`
	Local   string `json:"local"`
	Ordem   int    `json:"ordem"`
}

func (e *entradaSessao) validar() error {
	e.Titulo, e.Local = strings.TrimSpace(e.Titulo), strings.TrimSpace(e.Local)
	campos := map[string]string{}
	if e.Titulo == "" {
		campos["titulo"] = "Informe o título da sessão."
	}
	if !dataValida(e.Data) {
		campos["data"] = "Informe a data."
	}
	if !regexHorario.MatchString(e.Horario) {
		campos["horario"] = "Informe o horário (HH:MM)."
	}
	if e.Local == "" {
		campos["local"] = "Informe o local."
	}
	if len(campos) > 0 {
		return erroValidacao("Verifique os campos destacados.", campos)
	}
	return nil
}

func (s *Servidor) adminCriarSessao(w http.ResponseWriter, r *http.Request) error {
	var e entradaSessao
	if err := lerJSON(r, &e); err != nil {
		return err
	}
	if err := e.validar(); err != nil {
		return err
	}
	var id string
	if err := s.pool.QueryRow(r.Context(), `INSERT INTO sessoes_evento (evento_id, titulo, data, horario, local, ordem)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id::text`, chi.URLParam(r, "id"), e.Titulo, e.Data, e.Horario, e.Local, e.Ordem).Scan(&id); err != nil {
		return err
	}
	s.auditar(r.Context(), s.pool, r, "criar_sessao_evento", "sessao_evento", id, nil)
	responderJSON(w, http.StatusCreated, map[string]string{"id": id})
	return nil
}

func (s *Servidor) adminAtualizarSessao(w http.ResponseWriter, r *http.Request) error {
	var e entradaSessao
	if err := lerJSON(r, &e); err != nil {
		return err
	}
	if err := e.validar(); err != nil {
		return err
	}
	tag, err := s.pool.Exec(r.Context(), `UPDATE sessoes_evento SET titulo = $2, data = $3, horario = $4, local = $5, ordem = $6 WHERE id = $1`,
		chi.URLParam(r, "id"), e.Titulo, e.Data, e.Horario, e.Local, e.Ordem)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return erroNaoEncontrado("")
	}
	s.auditar(r.Context(), s.pool, r, "atualizar_sessao_evento", "sessao_evento", chi.URLParam(r, "id"),
		map[string]any{"data": e.Data, "horario": e.Horario, "local": e.Local})
	return responderOK(w, map[string]bool{"ok": true})
}

func (s *Servidor) adminRemoverSessao(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	var emUso bool
	if err := s.pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM trabalhos WHERE sessao_id = $1 AND estado <> 'cancelado')`, id).Scan(&emUso); err != nil {
		return err
	}
	if emUso {
		return erroConflito("Há trabalhos associados a esta sessão. Mova-os antes de removê-la.")
	}
	if _, err := s.pool.Exec(r.Context(), `DELETE FROM sessoes_evento WHERE id = $1`, id); err != nil {
		return err
	}
	s.auditar(r.Context(), s.pool, r, "remover_sessao_evento", "sessao_evento", id, nil)
	return responderOK(w, map[string]bool{"ok": true})
}

func (s *Servidor) adminAdicionarApresentador(w http.ResponseWriter, r *http.Request) error {
	eventoID := chi.URLParam(r, "id")
	var req struct {
		DocenteID    string `json:"docenteId"`
		PodePublicar *bool  `json:"podePublicar"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	if ok, err := possuiPerfil(r.Context(), s.pool, req.DocenteID, PerfilDocente); err != nil {
		return err
	} else if !ok {
		return erroRequisicao("Somente DOCENTES aprovados podem ser apresentadores.")
	}
	podePublicar := req.PodePublicar == nil || *req.PodePublicar
	ses := sessaoDe(r)
	if _, err := s.pool.Exec(r.Context(), `INSERT INTO apresentadores_evento (evento_id, docente_id, pode_publicar, atribuido_por)
		VALUES ($1, $2, $3, $4) ON CONFLICT (evento_id, docente_id) DO UPDATE SET pode_publicar = $3`,
		eventoID, req.DocenteID, podePublicar, ses.UsuarioID); err != nil {
		return err
	}
	s.auditar(r.Context(), s.pool, r, "atribuir_apresentador", "evento", eventoID, map[string]any{"docente": req.DocenteID, "podePublicar": podePublicar})
	return responderOK(w, map[string]bool{"ok": true})
}

func (s *Servidor) adminRemoverApresentador(w http.ResponseWriter, r *http.Request) error {
	eventoID, docenteID := chi.URLParam(r, "id"), chi.URLParam(r, "docenteId")
	if _, err := s.pool.Exec(r.Context(), `DELETE FROM apresentadores_evento WHERE evento_id = $1 AND docente_id = $2`, eventoID, docenteID); err != nil {
		return err
	}
	s.auditar(r.Context(), s.pool, r, "remover_apresentador", "evento", eventoID, map[string]any{"docente": docenteID})
	return responderOK(w, map[string]bool{"ok": true})
}

// ehApresentador verifica a atribuição de APRESENTADOR do docente naquele evento.
func ehApresentador(ctx context.Context, q banco.Executor, eventoID, docenteID string) (existe bool, podePublicar bool, err error) {
	err = q.QueryRow(ctx, `SELECT true, pode_publicar FROM apresentadores_evento WHERE evento_id = $1 AND docente_id = $2`, eventoID, docenteID).
		Scan(&existe, &podePublicar)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	return existe, podePublicar, err
}

// PUT .../eventos/{id}/programacao {trabalhoIds: [...]} — ADMIN ou APRESENTADOR do evento.
func (s *Servidor) definirProgramacao(w http.ResponseWriter, r *http.Request) error {
	eventoID := chi.URLParam(r, "id")
	ses := sessaoDe(r)
	if ses.Contexto == PerfilDocente {
		ok, _, err := ehApresentador(r.Context(), s.pool, eventoID, ses.UsuarioID)
		if err != nil {
			return err
		}
		if !ok {
			return erroProibido("Somente o apresentador do evento pode alterar a programação.")
		}
	}
	var req struct {
		TrabalhoIDs []string `json:"trabalhoIds"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	vistos := map[string]bool{}
	for i, id := range req.TrabalhoIDs {
		if vistos[id] {
			return erroRequisicao("Trabalho repetido na programação.")
		}
		vistos[id] = true
		tag, err := tx.Exec(r.Context(), `UPDATE trabalhos SET ordem_programacao = $3 WHERE id = $1 AND evento_id = $2`, id, eventoID, i+1)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return erroRequisicao("Há trabalhos que não pertencem a este evento.")
		}
	}
	if err := s.auditar(r.Context(), tx, r, "definir_programacao", "evento", eventoID, map[string]any{"ordem": req.TrabalhoIDs}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}

// associarEvento vincula o trabalho ao evento (e sessão, se houver). A data, o horário e o
// local da ATA virão sempre deste evento/sessão.
func associarEvento(ctx context.Context, tx pgx.Tx, trabalhoID, eventoID, sessaoID, atorID string, exigirFuturo bool) error {
	var possuiATA bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM atas WHERE trabalho_id = $1 AND estado <> 'cancelada')`, trabalhoID).Scan(&possuiATA); err != nil {
		return err
	}
	if possuiATA {
		return erroConflito("O trabalho já possui ATA. Para alterar o evento, reinicie o processo da ATA.")
	}
	if eventoID == "" {
		if _, err := tx.Exec(ctx, `UPDATE trabalhos SET evento_id = NULL, sessao_id = NULL, ordem_programacao = NULL, atualizado_em = now()
			WHERE id = $1`, trabalhoID); err != nil {
			return err
		}
		_, err := regras.RecalcularEstadoTrabalho(ctx, tx, trabalhoID, atorID, "Evento desassociado")
		return err
	}
	var totalSessoes int
	var futuro bool
	var estadoEvento string
	err := tx.QueryRow(ctx, `SELECT e.estado, (SELECT count(*) FROM sessoes_evento WHERE evento_id = e.id),
		(COALESCE(se.data, e.data_apresentacao) + COALESCE(se.horario, e.horario_apresentacao)) > (now() AT TIME ZONE 'America/Sao_Paulo')
		FROM eventos e LEFT JOIN sessoes_evento se ON se.id::text = $2 AND se.evento_id = e.id
		WHERE e.id = $1`, eventoID, sessaoID).Scan(&estadoEvento, &totalSessoes, &futuro)
	if errors.Is(err, pgx.ErrNoRows) {
		return erroNaoEncontrado("Evento não encontrado.")
	}
	if err != nil {
		return err
	}
	if totalSessoes > 0 && sessaoID == "" {
		return erroCampo("sessaoId", "Este evento possui sessões: escolha a sessão da apresentação.")
	}
	if sessaoID != "" {
		var pertence bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sessoes_evento WHERE id = $1 AND evento_id = $2)`, sessaoID, eventoID).Scan(&pertence); err != nil {
			return err
		}
		if !pertence {
			return erroCampo("sessaoId", "Sessão inválida para este evento.")
		}
	}
	if exigirFuturo {
		if !futuro {
			return erroConflito("Só é possível associar o trabalho a um evento cuja apresentação ainda não aconteceu.")
		}
		if estadoEvento == "rascunho" || estadoEvento == "encerrado" || estadoEvento == "publicado" {
			return erroConflito("Este evento não está recebendo trabalhos.")
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE trabalhos SET
		ordem_programacao = CASE WHEN evento_id IS DISTINCT FROM $2::uuid
			THEN (SELECT COALESCE(max(ordem_programacao), 0) + 1 FROM trabalhos WHERE evento_id = $2::uuid) ELSE ordem_programacao END,
		evento_id = $2, sessao_id = $3, atualizado_em = now() WHERE id = $1`, trabalhoID, eventoID, textoOuNil(sessaoID)); err != nil {
		return err
	}
	_, err = regras.RecalcularEstadoTrabalho(ctx, tx, trabalhoID, atorID, "Evento associado")
	return err
}

// POST /admin/trabalhos/{id}/evento — exceção administrativa para evento já ocorrido, com justificativa registrada.
func (s *Servidor) adminAssociarEventoExcecao(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	var req struct {
		EventoID      string `json:"eventoId"`
		SessaoID      string `json:"sessaoId"`
		Justificativa string `json:"justificativa"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	if len(strings.TrimSpace(req.Justificativa)) < 10 {
		return erroCampo("justificativa", "Descreva a justificativa da exceção (mínimo 10 caracteres).")
	}
	ses := sessaoDe(r)
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err := associarEvento(r.Context(), tx, id, req.EventoID, req.SessaoID, ses.UsuarioID, false); err != nil {
		return err
	}
	if _, err := tx.Exec(r.Context(), `UPDATE trabalhos SET excecao_evento_passado = $2 WHERE id = $1`, id, strings.TrimSpace(req.Justificativa)); err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "associar_evento_excecao", "trabalho", id,
		map[string]any{"evento": req.EventoID, "sessao": req.SessaoID, "justificativa": req.Justificativa}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}

// ---------------------------------------------------------------------
// Página pública: somente dados públicos (nunca ATA, notas, avaliações ou declarações).
// ---------------------------------------------------------------------

func (s *Servidor) publicoListarEventos(w http.ResponseWriter, r *http.Request) error {
	itens, err := listarEventos(r.Context(), s.pool, `WHERE e.estado <> 'rascunho' ORDER BY e.data_apresentacao DESC`)
	if err != nil {
		return err
	}
	r2 := make([]map[string]any, 0, len(itens))
	for _, e := range itens {
		r2 = append(r2, map[string]any{"nome": e.Nome, "edicao": e.Edicao, "descricao": e.Descricao, "dataApresentacao": e.DataApresentacao,
			"horarioApresentacao": e.HorarioApresentacao, "local": e.Local, "campus": e.Campus, "estado": e.Estado, "slug": e.Slug,
			"temCapa": e.TemCapa, "futuro": e.Futuro})
	}
	return responderOK(w, r2)
}

func (s *Servidor) publicoEvento(w http.ResponseWriter, r *http.Request) error {
	e, err := escanearEvento(s.pool.QueryRow(r.Context(), selecaoEvento+` WHERE e.slug = $1 AND e.estado <> 'rascunho'`, chi.URLParam(r, "slug")))
	if errors.Is(err, pgx.ErrNoRows) {
		return erroNaoEncontrado("Evento não encontrado.")
	}
	if err != nil {
		return err
	}
	sessoes, err := listarSessoes(r.Context(), s.pool, e.ID)
	if err != nil {
		return err
	}
	trabalhos, err := listarTrabalhosEvento(r.Context(), s.pool, e.ID)
	if err != nil {
		return err
	}
	programacao := make([]map[string]any, 0, len(trabalhos))
	for _, t := range trabalhos {
		if vazio(t.Titulo) {
			continue
		}
		programacao = append(programacao, map[string]any{"titulo": t.Titulo, "aluno": t.Aluno, "orientador": t.Orientador,
			"ordem": t.Ordem, "sessao": t.Sessao})
	}
	publicacoes, err := listarPublicacoes(r.Context(), s.pool, e.ID, true)
	if err != nil {
		return err
	}
	return responderOK(w, map[string]any{
		"evento": map[string]any{"nome": e.Nome, "edicao": e.Edicao, "descricao": e.Descricao, "dataApresentacao": e.DataApresentacao,
			"horarioApresentacao": e.HorarioApresentacao, "local": e.Local, "campus": e.Campus, "estado": e.Estado, "slug": e.Slug,
			"informacoesPublicas": e.InformacoesPublicas, "temCapa": e.TemCapa},
		"sessoes":     sessoes,
		"programacao": programacao,
		"publicacoes": publicacoes,
	})
}

func (s *Servidor) publicoCapa(w http.ResponseWriter, r *http.Request) error {
	var chave, mime *string
	err := s.pool.QueryRow(r.Context(), `SELECT capa_chave, capa_mime FROM eventos WHERE slug = $1 AND estado <> 'rascunho'`, chi.URLParam(r, "slug")).
		Scan(&chave, &mime)
	if err != nil || chave == nil {
		return erroNaoEncontrado("Capa não encontrada.")
	}
	return s.enviarArquivo(w, r, *chave, "capa", *mime, true)
}
