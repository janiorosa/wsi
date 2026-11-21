package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"wsi/internal/banco"
	"wsi/internal/fila"
)

var rotuloPublicacao = map[string]string{"anais": "Anais", "apresentacoes": "Apresentacoes"}

type itemPublicacaoResposta struct {
	Posicao   int     `json:"posicao"`
	Titulo    string  `json:"titulo"`
	Aluno     string  `json:"aluno"`
	Resultado string  `json:"resultado"`
	Detalhe   string  `json:"detalhe"`
	Paginas   *int    `json:"paginas"`
	Versao    *int    `json:"versaoArquivo"`
}

type publicacao struct {
	ID           string                   `json:"id"`
	Tipo         string                   `json:"tipo"`
	Versao       int                      `json:"versao"`
	Estado       string                   `json:"estado"`
	TotalPaginas *int                     `json:"totalPaginas"`
	Erro         string                   `json:"erro,omitempty"`
	GeradoEm     *string                  `json:"geradoEm"`
	GeradoPor    string                   `json:"geradoPor,omitempty"`
	PublicadoEm  *string                  `json:"publicadoEm"`
	PublicadoPor *string                  `json:"publicadoPor,omitempty"`
	Itens        []itemPublicacaoResposta `json:"itens,omitempty"`
}

// listarPublicacoes: com somentePublicadas=true, devolve apenas o que é público (sem relatório interno).
func listarPublicacoes(ctx context.Context, q banco.Executor, eventoID string, somentePublicadas bool) ([]publicacao, error) {
	rows, err := q.Query(ctx, `SELECT p.id::text, p.tipo, p.versao, p.estado, p.total_paginas, p.erro,
		to_char(p.gerado_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI'), g.nome_completo,
		to_char(p.publicado_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI'), pb.nome_completo
		FROM publicacoes_evento p JOIN usuarios g ON g.id = p.gerado_por LEFT JOIN usuarios pb ON pb.id = p.publicado_por
		WHERE p.evento_id = $1 AND (NOT $2 OR p.estado = 'publicado')
		ORDER BY p.tipo, p.versao DESC`, eventoID, somentePublicadas)
	if err != nil {
		return nil, err
	}
	itens := []publicacao{}
	for rows.Next() {
		var p publicacao
		if err := rows.Scan(&p.ID, &p.Tipo, &p.Versao, &p.Estado, &p.TotalPaginas, &p.Erro, &p.GeradoEm, &p.GeradoPor, &p.PublicadoEm, &p.PublicadoPor); err != nil {
			rows.Close()
			return nil, err
		}
		if somentePublicadas {
			p.GeradoPor, p.PublicadoPor, p.Erro = "", nil, ""
		}
		itens = append(itens, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if somentePublicadas {
		return itens, nil
	}
	for i := range itens {
		r, err := q.Query(ctx, `SELECT ip.posicao, t.titulo, u.nome_completo, ip.resultado, ip.detalhe, ip.paginas, af.versao
			FROM itens_publicacao ip JOIN trabalhos t ON t.id = ip.trabalho_id JOIN usuarios u ON u.id = t.aluno_id
			LEFT JOIN arquivos_trabalho af ON af.id = ip.arquivo_id
			WHERE ip.publicacao_id = $1 ORDER BY ip.posicao`, itens[i].ID)
		if err != nil {
			return nil, err
		}
		for r.Next() {
			var it itemPublicacaoResposta
			if err := r.Scan(&it.Posicao, &it.Titulo, &it.Aluno, &it.Resultado, &it.Detalhe, &it.Paginas, &it.Versao); err != nil {
				r.Close()
				return nil, err
			}
			itens[i].Itens = append(itens[i].Itens, it)
		}
		r.Close()
		if err := r.Err(); err != nil {
			return nil, err
		}
	}
	return itens, nil
}

func (s *Servidor) exigirApresentador(r *http.Request, eventoID string) (podePublicar bool, err error) {
	ok, pode, err := ehApresentador(r.Context(), s.pool, eventoID, sessaoDe(r).UsuarioID)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, erroNaoEncontrado("Evento não encontrado entre os seus eventos como apresentador.")
	}
	return pode, nil
}

func (s *Servidor) apresentadorListarEventos(w http.ResponseWriter, r *http.Request) error {
	itens, err := listarEventos(r.Context(), s.pool, `WHERE EXISTS (SELECT 1 FROM apresentadores_evento ap WHERE ap.evento_id = e.id AND ap.docente_id = $1)
		ORDER BY e.data_apresentacao DESC`, sessaoDe(r).UsuarioID)
	if err != nil {
		return err
	}
	return responderOK(w, itens)
}

// GET /apresentador/eventos/{id} — trabalhos, arquivos disponíveis e pendências. Sem notas e sem ATAs.
func (s *Servidor) apresentadorEvento(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	podePublicar, err := s.exigirApresentador(r, id)
	if err != nil {
		return err
	}
	e, err := escanearEvento(s.pool.QueryRow(r.Context(), selecaoEvento+` WHERE e.id = $1`, id))
	if err != nil {
		return err
	}
	rows, err := s.pool.Query(r.Context(), `SELECT t.id::text, t.titulo, a.nome_completo, t.ordem_programacao, se.titulo,
		(SELECT id::text FROM arquivos_trabalho WHERE trabalho_id = t.id AND tipo = 'artigo' ORDER BY versao DESC LIMIT 1),
		(SELECT versao FROM arquivos_trabalho WHERE trabalho_id = t.id AND tipo = 'artigo' ORDER BY versao DESC LIMIT 1),
		(SELECT id::text FROM arquivos_trabalho WHERE trabalho_id = t.id AND tipo = 'apresentacao' ORDER BY versao DESC LIMIT 1),
		(SELECT versao FROM arquivos_trabalho WHERE trabalho_id = t.id AND tipo = 'apresentacao' ORDER BY versao DESC LIMIT 1)
		FROM trabalhos t JOIN usuarios a ON a.id = t.aluno_id LEFT JOIN sessoes_evento se ON se.id = t.sessao_id
		WHERE t.evento_id = $1 AND t.estado <> 'cancelado'
		ORDER BY t.ordem_programacao NULLS LAST, t.titulo, t.id`, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	type trabalhoApresentador struct {
		ID                 string  `json:"id"`
		Titulo             string  `json:"titulo"`
		Aluno              string  `json:"aluno"`
		Ordem              *int    `json:"ordem"`
		Sessao             *string `json:"sessao"`
		ArtigoID           *string `json:"artigoId"`
		ArtigoVersao       *int    `json:"artigoVersao"`
		ApresentacaoID     *string `json:"apresentacaoId"`
		ApresentacaoVersao *int    `json:"apresentacaoVersao"`
	}
	trabalhos := []trabalhoApresentador{}
	for rows.Next() {
		var t trabalhoApresentador
		if err := rows.Scan(&t.ID, &t.Titulo, &t.Aluno, &t.Ordem, &t.Sessao, &t.ArtigoID, &t.ArtigoVersao, &t.ApresentacaoID, &t.ApresentacaoVersao); err != nil {
			return err
		}
		trabalhos = append(trabalhos, t)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	publicacoes, err := listarPublicacoes(r.Context(), s.pool, id, false)
	if err != nil {
		return err
	}
	var tarefasAtivas []map[string]string
	tr, err := s.pool.Query(r.Context(), `SELECT id::text, tipo, estado FROM tarefas_documentos
		WHERE entidade_id = $1 AND tipo IN ('gerar_anais', 'gerar_apresentacoes') AND estado IN ('pendente', 'processando')`, id)
	if err != nil {
		return err
	}
	for tr.Next() {
		var tid, tipo, estado string
		if err := tr.Scan(&tid, &tipo, &estado); err != nil {
			tr.Close()
			return err
		}
		tarefasAtivas = append(tarefasAtivas, map[string]string{"id": tid, "tipo": tipo, "estado": estado})
	}
	tr.Close()
	if tarefasAtivas == nil {
		tarefasAtivas = []map[string]string{}
	}
	return responderOK(w, map[string]any{"evento": e, "trabalhos": trabalhos, "publicacoes": publicacoes,
		"tarefasAtivas": tarefasAtivas, "podePublicar": podePublicar})
}

// POST /apresentador/eventos/{id}/publicacoes {tipo: anais|apresentacoes}
func (s *Servidor) apresentadorGerarPublicacao(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	if _, err := s.exigirApresentador(r, id); err != nil {
		return err
	}
	var req struct {
		Tipo string `json:"tipo"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	tipoTarefa := map[string]string{"anais": fila.TipoAnais, "apresentacoes": fila.TipoApresentacoes}[req.Tipo]
	if tipoTarefa == "" {
		return erroCampo("tipo", "Tipo de publicação inválido.")
	}
	ses := sessaoDe(r)
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	// Serializa por evento: duplo clique não cria duas compilações.
	if _, err := tx.Exec(ctx, `SELECT id FROM eventos WHERE id = $1 FOR UPDATE`, id); err != nil {
		return err
	}
	var tarefaExistente, publicacaoExistente string
	err = tx.QueryRow(ctx, `SELECT id::text, parametros->>'publicacao_id' FROM tarefas_documentos
		WHERE tipo = $1 AND entidade_id = $2 AND estado IN ('pendente', 'processando')`, tipoTarefa, id).Scan(&tarefaExistente, &publicacaoExistente)
	if err == nil {
		responderJSON(w, http.StatusAccepted, map[string]string{"tarefaId": tarefaExistente, "publicacaoId": publicacaoExistente})
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var publicacaoID string
	if err := tx.QueryRow(ctx, `INSERT INTO publicacoes_evento (evento_id, tipo, versao, estado, gerado_por)
		VALUES ($1, $2, (SELECT COALESCE(max(versao), 0) + 1 FROM publicacoes_evento WHERE evento_id = $1 AND tipo = $2), 'em_preparacao', $3)
		RETURNING id::text`, id, req.Tipo, ses.UsuarioID).Scan(&publicacaoID); err != nil {
		return err
	}
	tarefaID, err := fila.Enfileirar(ctx, tx, tipoTarefa, id, map[string]string{"publicacao_id": publicacaoID}, ses.UsuarioID)
	if err != nil {
		return err
	}
	if err := s.auditar(ctx, tx, r, "gerar_"+req.Tipo, "evento", id, map[string]any{"publicacao": publicacaoID}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.fila.Notificar()
	responderJSON(w, http.StatusAccepted, map[string]string{"tarefaId": tarefaID, "publicacaoId": publicacaoID})
	return nil
}

func (s *Servidor) carregarPublicacao(ctx context.Context, id string) (eventoID, tipo, estado string, chave *string, versao int, nomeEvento string, err error) {
	err = s.pool.QueryRow(ctx, `SELECT p.evento_id::text, p.tipo, p.estado, p.chave_armazenamento, p.versao, e.nome
		FROM publicacoes_evento p JOIN eventos e ON e.id = p.evento_id WHERE p.id = $1`, id).Scan(&eventoID, &tipo, &estado, &chave, &versao, &nomeEvento)
	if errors.Is(err, pgx.ErrNoRows) {
		err = erroNaoEncontrado("Publicação não encontrada.")
	}
	return
}

// GET /apresentador/publicacoes/{id}/arquivo — conferência antes da publicação.
func (s *Servidor) apresentadorBaixarPublicacao(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	eventoID, tipo, _, chave, versao, nome, err := s.carregarPublicacao(r.Context(), id)
	if err != nil {
		return err
	}
	if _, err := s.exigirApresentador(r, eventoID); err != nil {
		return err
	}
	if chave == nil {
		return erroConflito("O arquivo ainda não foi gerado.")
	}
	return s.enviarArquivo(w, r, *chave, nomeDownload(rotuloPublicacao[tipo], nome, "v"+itoa(versao)), "application/pdf", r.URL.Query().Get("visualizar") == "1")
}

// POST /apresentador/publicacoes/{id}/publicar — ação explícita do APRESENTADOR; sem aprovação adicional do ADMIN.
func (s *Servidor) apresentadorPublicar(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	eventoID, tipo, estado, _, _, _, err := s.carregarPublicacao(r.Context(), id)
	if err != nil {
		return err
	}
	pode, err := s.exigirApresentador(r, eventoID)
	if err != nil {
		return err
	}
	if !pode {
		return erroProibido("Você não recebeu permissão para publicar os documentos deste evento.")
	}
	if estado != "aguardando_revisao" {
		return erroConflito("Somente documentos gerados e aguardando revisão podem ser publicados.")
	}
	ses := sessaoDe(r)
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SELECT id FROM eventos WHERE id = $1 FOR UPDATE`, eventoID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE publicacoes_evento SET estado = 'substituido' WHERE evento_id = $1 AND tipo = $2 AND estado = 'publicado'`,
		eventoID, tipo); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE publicacoes_evento SET estado = 'publicado', publicado_por = $2, publicado_em = now()
		WHERE id = $1 AND estado = 'aguardando_revisao'`, id, ses.UsuarioID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return erroConflito("A publicação mudou de estado. Recarregue a página.")
	}
	if err := s.auditar(ctx, tx, r, "publicar_"+tipo, "evento", eventoID, map[string]any{"publicacao": id}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}

// GET /publico/publicacoes/{id}/arquivo — somente publicações no estado "publicado".
func (s *Servidor) publicoArquivoPublicacao(w http.ResponseWriter, r *http.Request) error {
	_, tipo, estado, chave, versao, nome, err := s.carregarPublicacao(r.Context(), chi.URLParam(r, "id"))
	if err != nil || estado != "publicado" || chave == nil {
		return erroNaoEncontrado("Publicação não encontrada.")
	}
	return s.enviarArquivo(w, r, *chave, nomeDownload(rotuloPublicacao[tipo], nome, "v"+itoa(versao)), "application/pdf", r.URL.Query().Get("visualizar") == "1")
}
