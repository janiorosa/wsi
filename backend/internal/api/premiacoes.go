package api

import (
	"context"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"wsi/internal/premiacoes"
)

// Votação aberta enquanto o evento não for rascunho nem estiver publicado.
const condicaoVotacaoAberta = `e.estado NOT IN ('rascunho', 'publicado')`

type entradaPremiacao struct {
	Nome      string `json:"nome"`
	Descricao string `json:"descricao"`
}

func (p *entradaPremiacao) validar() error {
	p.Nome = strings.Join(strings.Fields(p.Nome), " ")
	p.Descricao = strings.TrimSpace(p.Descricao)
	if p.Nome == "" {
		return erroCampo("nome", "Informe o nome da premiação.")
	}
	if utf8.RuneCountInString(p.Nome) > 200 {
		return erroCampo("nome", "O nome deve ter no máximo 200 caracteres.")
	}
	return nil
}

// ---------------------------------------------------------------------
// ADMIN: premiações por evento
// ---------------------------------------------------------------------

// POST /admin/eventos/{id}/premiacoes {nome, descricao}
func (s *Servidor) adminCriarPremiacao(w http.ResponseWriter, r *http.Request) error {
	eventoID := chi.URLParam(r, "id")
	var req entradaPremiacao
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	if err := req.validar(); err != nil {
		return err
	}
	ses := sessaoDe(r)
	var id string
	if err := s.pool.QueryRow(r.Context(), `INSERT INTO premiacoes (evento_id, nome, descricao, ordem, criado_por)
		VALUES ($1, $2, $3, (SELECT COALESCE(max(ordem), 0) + 1 FROM premiacoes WHERE evento_id = $1), $4) RETURNING id::text`,
		eventoID, req.Nome, req.Descricao, ses.UsuarioID).Scan(&id); err != nil {
		return err
	}
	s.auditar(r.Context(), s.pool, r, "criar_premiacao", "evento", eventoID, map[string]any{"premiacao": id, "nome": req.Nome})
	responderJSON(w, http.StatusCreated, map[string]string{"id": id})
	return nil
}

// PUT /admin/premiacoes/{id} {nome, descricao}
func (s *Servidor) adminAtualizarPremiacao(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	var req entradaPremiacao
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	if err := req.validar(); err != nil {
		return err
	}
	var eventoID string
	if err := s.pool.QueryRow(r.Context(), `UPDATE premiacoes SET nome = $2, descricao = $3 WHERE id = $1 RETURNING evento_id::text`,
		id, req.Nome, req.Descricao).Scan(&eventoID); err != nil {
		return err
	}
	s.auditar(r.Context(), s.pool, r, "atualizar_premiacao", "evento", eventoID, map[string]any{"premiacao": id, "nome": req.Nome})
	return responderOK(w, map[string]bool{"ok": true})
}

// DELETE /admin/premiacoes/{id} — remove a premiação e os votos dela.
func (s *Servidor) adminRemoverPremiacao(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	var eventoID, nome string
	if err := s.pool.QueryRow(r.Context(), `DELETE FROM premiacoes WHERE id = $1 RETURNING evento_id::text, nome`, id).Scan(&eventoID, &nome); err != nil {
		return err
	}
	s.auditar(r.Context(), s.pool, r, "remover_premiacao", "evento", eventoID, map[string]any{"premiacao": id, "nome": nome})
	return responderOK(w, map[string]bool{"ok": true})
}

// ---------------------------------------------------------------------
// DOCENTE: votação (até dois trabalhos por premiação)
// ---------------------------------------------------------------------

type trabalhoVotacao struct {
	ID         string  `json:"id"`
	Titulo     string  `json:"titulo"`
	Aluno      string  `json:"aluno"`
	Orientador *string `json:"orientador"`
}

type premiacaoVotacao struct {
	ID        string   `json:"id"`
	Nome      string   `json:"nome"`
	Descricao string   `json:"descricao"`
	MeusVotos []string `json:"meusVotos"`
}

type eventoVotacao struct {
	ID         string             `json:"id"`
	Nome       string             `json:"nome"`
	Edicao     string             `json:"edicao"`
	Data       string             `json:"data"`
	Trabalhos  []trabalhoVotacao  `json:"trabalhos"`
	Premiacoes []premiacaoVotacao `json:"premiacoes"`
}

// consultarVotacoes: eventos com votação aberta, seus trabalhos e os votos do docente.
func (s *Servidor) consultarVotacoes(ctx context.Context, docenteID string) ([]eventoVotacao, error) {
	rows, err := s.pool.Query(ctx, `SELECT e.id::text, e.nome, e.edicao, to_char(e.data_apresentacao, 'DD/MM/YYYY')
		FROM eventos e WHERE `+condicaoVotacaoAberta+`
		  AND EXISTS (SELECT 1 FROM premiacoes p WHERE p.evento_id = e.id)
		  AND EXISTS (SELECT 1 FROM trabalhos t WHERE t.evento_id = e.id AND t.estado <> 'cancelado' AND trim(t.titulo) <> '')
		ORDER BY e.data_apresentacao, e.nome`)
	if err != nil {
		return nil, err
	}
	eventos := []eventoVotacao{}
	for rows.Next() {
		var e eventoVotacao
		if err := rows.Scan(&e.ID, &e.Nome, &e.Edicao, &e.Data); err != nil {
			rows.Close()
			return nil, err
		}
		e.Trabalhos, e.Premiacoes = []trabalhoVotacao{}, []premiacaoVotacao{}
		eventos = append(eventos, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range eventos {
		tr, err := s.pool.Query(ctx, `SELECT t.id::text, t.titulo, a.nome_completo, o.nome_completo
			FROM trabalhos t JOIN usuarios a ON a.id = t.aluno_id LEFT JOIN usuarios o ON o.id = t.orientador_id
			WHERE t.evento_id = $1 AND t.estado <> 'cancelado' AND trim(t.titulo) <> ''
			ORDER BY t.ordem_programacao NULLS LAST, t.titulo, t.id`, eventos[i].ID)
		if err != nil {
			return nil, err
		}
		for tr.Next() {
			var t trabalhoVotacao
			if err := tr.Scan(&t.ID, &t.Titulo, &t.Aluno, &t.Orientador); err != nil {
				tr.Close()
				return nil, err
			}
			eventos[i].Trabalhos = append(eventos[i].Trabalhos, t)
		}
		tr.Close()
		if err := tr.Err(); err != nil {
			return nil, err
		}
		pr, err := s.pool.Query(ctx, `SELECT p.id::text, p.nome, p.descricao,
			COALESCE((SELECT array_agg(v.trabalho_id::text ORDER BY v.votado_em) FROM votos_premiacao v
			          WHERE v.premiacao_id = p.id AND v.docente_id = $2), '{}'::text[])
			FROM premiacoes p WHERE p.evento_id = $1 ORDER BY p.ordem, p.nome`, eventos[i].ID, docenteID)
		if err != nil {
			return nil, err
		}
		for pr.Next() {
			var p premiacaoVotacao
			if err := pr.Scan(&p.ID, &p.Nome, &p.Descricao, &p.MeusVotos); err != nil {
				pr.Close()
				return nil, err
			}
			eventos[i].Premiacoes = append(eventos[i].Premiacoes, p)
		}
		pr.Close()
		if err := pr.Err(); err != nil {
			return nil, err
		}
	}
	return eventos, nil
}

// GET /docente/premiacoes
func (s *Servidor) docenteListarVotacoes(w http.ResponseWriter, r *http.Request) error {
	itens, err := s.consultarVotacoes(r.Context(), sessaoDe(r).UsuarioID)
	if err != nil {
		return err
	}
	return responderOK(w, itens)
}

// POST /docente/premiacoes/{id}/votos {trabalhoId}
func (s *Servidor) docenteVotar(w http.ResponseWriter, r *http.Request) error {
	premiacaoID := chi.URLParam(r, "id")
	var req struct {
		TrabalhoID string `json:"trabalhoId"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	ses := sessaoDe(r)
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	// Bloqueia a premiação: votos simultâneos do mesmo docente não ultrapassam o limite.
	var eventoID string
	var aberta bool
	if err := tx.QueryRow(ctx, `SELECT p.evento_id::text, `+condicaoVotacaoAberta+`
		FROM premiacoes p JOIN eventos e ON e.id = p.evento_id WHERE p.id = $1 FOR UPDATE OF p`, premiacaoID).Scan(&eventoID, &aberta); err != nil {
		return erroNaoEncontrado("Premiação não encontrada.")
	}
	if !aberta {
		return erroConflito("A votação desta premiação está encerrada.")
	}
	var valido bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM trabalhos WHERE id::text = $1 AND evento_id = $2 AND estado <> 'cancelado')`,
		req.TrabalhoID, eventoID).Scan(&valido); err != nil {
		return err
	}
	if !valido {
		return erroCampo("trabalhoId", "Trabalho não encontrado neste evento.")
	}
	var jaVotou bool
	var meusVotos int
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM votos_premiacao WHERE premiacao_id = $1 AND docente_id = $2 AND trabalho_id::text = $3),
		(SELECT count(*) FROM votos_premiacao WHERE premiacao_id = $1 AND docente_id = $2)`,
		premiacaoID, ses.UsuarioID, req.TrabalhoID).Scan(&jaVotou, &meusVotos); err != nil {
		return err
	}
	if jaVotou {
		return responderOK(w, map[string]bool{"ok": true})
	}
	if meusVotos >= premiacoes.MaximoVotosPorDocente {
		return erroConflito("Você já votou em dois trabalhos nesta premiação. Retire um voto para escolher outro trabalho.")
	}
	if _, err := tx.Exec(ctx, `INSERT INTO votos_premiacao (premiacao_id, docente_id, trabalho_id) VALUES ($1, $2, $3)`,
		premiacaoID, ses.UsuarioID, req.TrabalhoID); err != nil {
		return err
	}
	if err := s.auditar(ctx, tx, r, "votar_premiacao", "evento", eventoID, map[string]any{"premiacao": premiacaoID, "trabalho": req.TrabalhoID}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}

// DELETE /docente/premiacoes/{id}/votos/{trabalhoId}
func (s *Servidor) docenteRemoverVoto(w http.ResponseWriter, r *http.Request) error {
	premiacaoID, trabalhoID := chi.URLParam(r, "id"), chi.URLParam(r, "trabalhoId")
	ses := sessaoDe(r)
	var eventoID string
	var aberta bool
	if err := s.pool.QueryRow(r.Context(), `SELECT p.evento_id::text, `+condicaoVotacaoAberta+`
		FROM premiacoes p JOIN eventos e ON e.id = p.evento_id WHERE p.id = $1`, premiacaoID).Scan(&eventoID, &aberta); err != nil {
		return erroNaoEncontrado("Premiação não encontrada.")
	}
	if !aberta {
		return erroConflito("A votação desta premiação está encerrada.")
	}
	if _, err := s.pool.Exec(r.Context(), `DELETE FROM votos_premiacao WHERE premiacao_id = $1 AND docente_id = $2 AND trabalho_id::text = $3`,
		premiacaoID, ses.UsuarioID, trabalhoID); err != nil {
		return err
	}
	s.auditar(r.Context(), s.pool, r, "remover_voto_premiacao", "evento", eventoID, map[string]any{"premiacao": premiacaoID, "trabalho": trabalhoID})
	return responderOK(w, map[string]bool{"ok": true})
}
