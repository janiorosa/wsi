package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"wsi/internal/banco"
	"wsi/internal/notas"
	"wsi/internal/regras"
)

// ---------------------------------------------------------------------
// Critérios do trabalho (fotografia histórica)
// ---------------------------------------------------------------------

type criterioTrabalho struct {
	ID         string `json:"id"`
	Nome       string `json:"nome"`
	Descricao  string `json:"descricao"`
	NotaMinima int    `json:"notaMinima"`
	NotaMaxima int    `json:"notaMaxima"`
	Origem     string `json:"origem"`
	Ordem      int    `json:"ordem"`
}

// criteriosBloqueados: após a primeira avaliação finalizada, os critérios do trabalho ficam congelados.
func criteriosBloqueados(ctx context.Context, q banco.Executor, trabalhoID string) (bool, error) {
	var b bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM avaliacoes WHERE trabalho_id = $1 AND estado = 'finalizada')`, trabalhoID).Scan(&b)
	return b, err
}

func exigirCriteriosEditaveis(ctx context.Context, q banco.Executor, trabalhoID string) error {
	b, err := criteriosBloqueados(ctx, q, trabalhoID)
	if err != nil {
		return err
	}
	if b {
		return erroConflito("Os critérios não podem mais ser alterados: já existe avaliação finalizada para este trabalho.")
	}
	return nil
}

// sincronizarCriteriosGerais copia para o trabalho os critérios gerais ativos (e retira os inativados),
// enquanto nenhuma avaliação tiver sido finalizada. Depois disso, nada muda.
func sincronizarCriteriosGerais(ctx context.Context, q banco.Executor, trabalhoID string) error {
	if b, err := criteriosBloqueados(ctx, q, trabalhoID); err != nil || b {
		return err
	}
	if _, err := q.Exec(ctx, `DELETE FROM notas_avaliacao WHERE criterio_trabalho_id IN (
		SELECT ct.id FROM criterios_trabalho ct JOIN criterios_avaliacao c ON c.id = ct.criterio_geral_id
		WHERE ct.trabalho_id = $1 AND NOT c.ativo)`, trabalhoID); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `DELETE FROM criterios_trabalho ct USING criterios_avaliacao c
		WHERE ct.trabalho_id = $1 AND c.id = ct.criterio_geral_id AND NOT c.ativo`, trabalhoID); err != nil {
		return err
	}
	// Atualiza nome/descrição dos gerais ainda não congelados e insere os que faltam.
	if _, err := q.Exec(ctx, `UPDATE criterios_trabalho ct SET nome = c.nome, descricao = c.descricao, ordem = c.ordem
		FROM criterios_avaliacao c WHERE ct.trabalho_id = $1 AND c.id = ct.criterio_geral_id`, trabalhoID); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `INSERT INTO criterios_trabalho (trabalho_id, criterio_geral_id, origem, nome, descricao, ordem)
		SELECT $1, c.id, 'geral', c.nome, c.descricao, c.ordem FROM criterios_avaliacao c
		WHERE c.ativo AND NOT EXISTS (SELECT 1 FROM criterios_trabalho ct WHERE ct.trabalho_id = $1 AND ct.criterio_geral_id = c.id)`, trabalhoID)
	return err
}

func listarCriteriosTrabalho(ctx context.Context, q banco.Executor, trabalhoID string) ([]criterioTrabalho, bool, error) {
	bloqueados, err := criteriosBloqueados(ctx, q, trabalhoID)
	if err != nil {
		return nil, false, err
	}
	rows, err := q.Query(ctx, `SELECT id::text, nome, descricao, nota_minima::int, nota_maxima::int, origem, ordem
		FROM criterios_trabalho WHERE trabalho_id = $1 ORDER BY CASE origem WHEN 'geral' THEN 0 ELSE 1 END, ordem, nome`, trabalhoID)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	itens := []criterioTrabalho{}
	for rows.Next() {
		var c criterioTrabalho
		if err := rows.Scan(&c.ID, &c.Nome, &c.Descricao, &c.NotaMinima, &c.NotaMaxima, &c.Origem, &c.Ordem); err != nil {
			return nil, false, err
		}
		itens = append(itens, c)
	}
	return itens, bloqueados, rows.Err()
}

// ---------------------------------------------------------------------
// Avaliações do docente
// ---------------------------------------------------------------------

type itemAvaliacao struct {
	TrabalhoID      string  `json:"trabalhoId"`
	Titulo          string  `json:"titulo"`
	Aluno           string  `json:"aluno"`
	Evento          *string `json:"evento"`
	DataEvento      *string `json:"dataEvento"`
	Papel           string  `json:"papel"`
	EstadoAvaliacao string  `json:"estadoAvaliacao"`
	Nota            *string `json:"nota"`
	EstadoTrabalho  string  `json:"estadoTrabalho"`
}

// consultarAvaliacoesDocente: somente trabalhos para os quais o docente foi designado (inclui os dos seus orientandos).
func (s *Servidor) consultarAvaliacoesDocente(ctx context.Context, docenteID string) ([]itemAvaliacao, error) {
	rows, err := s.pool.Query(ctx, `SELECT t.id::text, t.titulo, a.nome_completo, e.nome, to_char(e.data_apresentacao, 'DD/MM/YYYY'), mb.papel,
		COALESCE(av.estado, 'pendente'), av.nota_calculada::text, t.estado
		FROM membros_banca mb JOIN trabalhos t ON t.id = mb.trabalho_id JOIN usuarios a ON a.id = t.aluno_id
		LEFT JOIN eventos e ON e.id = t.evento_id
		LEFT JOIN avaliacoes av ON av.trabalho_id = t.id AND av.avaliador_id = mb.docente_id
		WHERE mb.docente_id = $1 AND mb.situacao = 'ativo' AND t.estado <> 'cancelado'
		ORDER BY (COALESCE(av.estado, 'pendente') = 'finalizada'), e.data_apresentacao NULLS LAST, t.titulo`, docenteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	itens := []itemAvaliacao{}
	for rows.Next() {
		var a itemAvaliacao
		if err := rows.Scan(&a.TrabalhoID, &a.Titulo, &a.Aluno, &a.Evento, &a.DataEvento, &a.Papel, &a.EstadoAvaliacao, &a.Nota, &a.EstadoTrabalho); err != nil {
			return nil, err
		}
		if a.Nota != nil {
			if c, err := notas.DeTextoNumeric(*a.Nota); err == nil {
				f := c.FormatarBR()
				a.Nota = &f
			}
		}
		itens = append(itens, a)
	}
	return itens, rows.Err()
}

func (s *Servidor) docenteListarAvaliacoes(w http.ResponseWriter, r *http.Request) error {
	itens, err := s.consultarAvaliacoesDocente(r.Context(), sessaoDe(r).UsuarioID)
	if err != nil {
		return err
	}
	return responderOK(w, itens)
}

func exigirMembroBanca(ctx context.Context, q banco.Executor, trabalhoID, docenteID string) error {
	var membro bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM membros_banca WHERE trabalho_id = $1 AND docente_id = $2 AND situacao = 'ativo')`,
		trabalhoID, docenteID).Scan(&membro); err != nil {
		return err
	}
	if !membro {
		return erroNaoEncontrado("Você não foi designado para avaliar este trabalho.")
	}
	return nil
}

type criterioComNota struct {
	criterioTrabalho
	Nota       *float64 `json:"nota"`
	Comentario string   `json:"comentario"`
}

// GET /docente/avaliacoes/{trabalhoId} — tela única de avaliação.
func (s *Servidor) docenteObterAvaliacao(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	ctx := r.Context()
	trabalhoID := chi.URLParam(r, "trabalhoId")
	if err := exigirMembroBanca(ctx, s.pool, trabalhoID, ses.UsuarioID); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err := sincronizarCriteriosGerais(ctx, tx, trabalhoID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	var titulo, resumo, aluno, papel string
	var artigoID *string
	if err := s.pool.QueryRow(ctx, `SELECT t.titulo, t.resumo, a.nome_completo, mb.papel,
		(SELECT id::text FROM arquivos_trabalho WHERE trabalho_id = t.id AND tipo = 'artigo' ORDER BY versao DESC LIMIT 1)
		FROM trabalhos t JOIN usuarios a ON a.id = t.aluno_id
		JOIN membros_banca mb ON mb.trabalho_id = t.id AND mb.docente_id = $2 AND mb.situacao = 'ativo'
		WHERE t.id = $1`, trabalhoID, ses.UsuarioID).Scan(&titulo, &resumo, &aluno, &papel, &artigoID); err != nil {
		return err
	}
	evento, err := eventoDoTrabalho(ctx, s.pool, trabalhoID)
	if err != nil {
		return err
	}
	criterios, _, err := listarCriteriosTrabalho(ctx, s.pool, trabalhoID)
	if err != nil {
		return err
	}
	estado, notaCalculada := "pendente", (*string)(nil)
	var avaliacaoID, parecer string
	var parecerEm *string
	err = s.pool.QueryRow(ctx, `SELECT id::text, estado, nota_calculada::text, parecer,
		to_char(parecer_atualizado_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI')
		FROM avaliacoes WHERE trabalho_id = $1 AND avaliador_id = $2`,
		trabalhoID, ses.UsuarioID).Scan(&avaliacaoID, &estado, &notaCalculada, &parecer, &parecerEm)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var possuiATA bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM atas WHERE trabalho_id = $1 AND estado <> 'cancelada')`, trabalhoID).Scan(&possuiATA); err != nil {
		return err
	}
	notasExistentes := map[string]criterioComNota{}
	if avaliacaoID != "" {
		rows, err := s.pool.Query(ctx, `SELECT criterio_trabalho_id::text, nota::text, comentario FROM notas_avaliacao WHERE avaliacao_id = $1`, avaliacaoID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var cid, nota, comentario string
			if err := rows.Scan(&cid, &nota, &comentario); err != nil {
				rows.Close()
				return err
			}
			c, _ := notas.DeTextoNumeric(nota)
			f := c.Float()
			notasExistentes[cid] = criterioComNota{Nota: &f, Comentario: comentario}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
	}
	itens := make([]criterioComNota, 0, len(criterios))
	for _, c := range criterios {
		item := notasExistentes[c.ID]
		item.criterioTrabalho = c
		itens = append(itens, item)
	}
	if notaCalculada != nil {
		if c, err := notas.DeTextoNumeric(*notaCalculada); err == nil {
			f := c.FormatarBR()
			notaCalculada = &f
		}
	}
	return responderOK(w, map[string]any{
		"trabalho":      map[string]any{"id": trabalhoID, "titulo": titulo, "resumo": resumo, "aluno": aluno, "artigoId": artigoID},
		"evento":        evento,
		"papel":         papel,
		"criterios":     itens,
		"estado":        estado,
		"notaCalculada": notaCalculada,
		"podeEditar":    estado != "finalizada" && !possuiATA,
		"regra":         "Média aritmética simples das notas dos critérios (0 a 100), sem pesos, arredondada a 2 casas.",
		// Parecer descritivo: informativo para o aluno; independente da nota e editável até a geração da ATA.
		"parecer":             parecer,
		"parecerAtualizadoEm": parecerEm,
		"podeEditarParecer":   !possuiATA,
	})
}

// PUT /docente/avaliacoes/{trabalhoId} {notas: [{criterioId, nota, comentario}], finalizar}
func (s *Servidor) docenteSalvarAvaliacao(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	ctx := r.Context()
	trabalhoID := chi.URLParam(r, "trabalhoId")
	var req struct {
		Notas []struct {
			CriterioID string   `json:"criterioId"`
			Nota       *float64 `json:"nota"`
			Comentario string   `json:"comentario"`
		} `json:"notas"`
		Finalizar bool `json:"finalizar"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SELECT id FROM trabalhos WHERE id = $1 FOR UPDATE`, trabalhoID); err != nil {
		return err
	}
	if err := exigirMembroBanca(ctx, tx, trabalhoID, ses.UsuarioID); err != nil {
		return err
	}
	var possuiATA bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM atas WHERE trabalho_id = $1 AND estado <> 'cancelada')`, trabalhoID).Scan(&possuiATA); err != nil {
		return err
	}
	if possuiATA {
		return erroConflito("A ATA deste trabalho já foi gerada; as avaliações estão encerradas.")
	}
	if err := sincronizarCriteriosGerais(ctx, tx, trabalhoID); err != nil {
		return err
	}
	criterios, _, err := listarCriteriosTrabalho(ctx, tx, trabalhoID)
	if err != nil {
		return err
	}
	if len(criterios) == 0 {
		return erroConflito("Nenhum critério de avaliação está configurado. Solicite à administração o cadastro do critério geral.")
	}
	validos := map[string]bool{}
	for _, c := range criterios {
		validos[c.ID] = true
	}

	var avaliacaoID, estado string
	err = tx.QueryRow(ctx, `INSERT INTO avaliacoes (trabalho_id, avaliador_id) VALUES ($1, $2)
		ON CONFLICT (trabalho_id, avaliador_id) DO UPDATE SET atualizado_em = now()
		RETURNING id::text, estado`, trabalhoID, ses.UsuarioID).Scan(&avaliacaoID, &estado)
	if err != nil {
		return err
	}
	if estado == "finalizada" {
		return erroConflito("Esta avaliação já foi finalizada. Para alterá-la, o orientador deve reabri-la.")
	}

	campos := map[string]string{}
	notasPorCriterio := map[string]notas.Centesimos{}
	for _, n := range req.Notas {
		if !validos[n.CriterioID] {
			return erroRequisicao("Critério inválido para este trabalho.")
		}
		if n.Nota == nil {
			if _, err := tx.Exec(ctx, `DELETE FROM notas_avaliacao WHERE avaliacao_id = $1 AND criterio_trabalho_id = $2`, avaliacaoID, n.CriterioID); err != nil {
				return err
			}
			continue
		}
		c, err := notas.DeDecimal(*n.Nota)
		if err != nil {
			campos[n.CriterioID] = "Nota inválida: use um valor de 0 a 100 com até duas casas decimais."
			continue
		}
		notasPorCriterio[n.CriterioID] = c
		if _, err := tx.Exec(ctx, `INSERT INTO notas_avaliacao (avaliacao_id, criterio_trabalho_id, nota, comentario) VALUES ($1, $2, $3, $4)
			ON CONFLICT (avaliacao_id, criterio_trabalho_id) DO UPDATE SET nota = $3, comentario = $4`,
			avaliacaoID, n.CriterioID, c.Decimal(), strings.TrimSpace(n.Comentario)); err != nil {
			return err
		}
	}
	if len(campos) > 0 {
		return erroValidacao("Há notas fora da escala de 0 a 100.", campos)
	}

	resposta := map[string]any{"estado": "rascunho"}
	if req.Finalizar {
		// Todas as notas devem estar preenchidas (inclusive as já gravadas anteriormente).
		rows, err := tx.Query(ctx, `SELECT ct.id::text, na.nota::text FROM criterios_trabalho ct
			LEFT JOIN notas_avaliacao na ON na.criterio_trabalho_id = ct.id AND na.avaliacao_id = $2
			WHERE ct.trabalho_id = $1`, trabalhoID, avaliacaoID)
		if err != nil {
			return err
		}
		var valores []notas.Centesimos
		faltando := map[string]string{}
		for rows.Next() {
			var cid string
			var nota *string
			if err := rows.Scan(&cid, &nota); err != nil {
				rows.Close()
				return err
			}
			if nota == nil {
				faltando[cid] = "Informe a nota deste critério."
				continue
			}
			c, err := notas.DeTextoNumeric(*nota)
			if err != nil {
				rows.Close()
				return err
			}
			valores = append(valores, c)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(faltando) > 0 {
			return erroValidacao("Preencha as notas de todos os critérios antes de confirmar.", faltando)
		}
		nota, err := notas.NotaAvaliador(valores)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE avaliacoes SET estado = 'finalizada', nota_calculada = $2, enviada_em = now(), versao_regra = $3,
			atualizado_em = now() WHERE id = $1`, avaliacaoID, nota.Decimal(), notas.VersaoRegra); err != nil {
			return err
		}
		novoEstado, err := regras.RecalcularEstadoTrabalho(ctx, tx, trabalhoID, ses.UsuarioID, "Avaliação finalizada")
		if err != nil {
			return err
		}
		if err := s.auditar(ctx, tx, r, "finalizar_avaliacao", "trabalho", trabalhoID, map[string]any{"nota": nota.Decimal()}); err != nil {
			return err
		}
		resposta = map[string]any{"estado": "finalizada", "nota": nota.FormatarBR()}
		// Todas as avaliações obrigatórias concluídas e evento associado: a ATA é gerada automaticamente.
		if novoEstado == regras.EstadoAvaliacoesConcluidas {
			var temEvento bool
			if err := tx.QueryRow(ctx, `SELECT evento_id IS NOT NULL FROM trabalhos WHERE id = $1`, trabalhoID).Scan(&temEvento); err != nil {
				return err
			}
			if temEvento {
				ataID, tarefaID, err := iniciarGeracaoATA(ctx, tx, trabalhoID, ses.UsuarioID)
				if err != nil {
					return err
				}
				resposta["ataId"], resposta["tarefaId"] = ataID, tarefaID
			}
		}
	} else {
		if _, err := tx.Exec(ctx, `UPDATE avaliacoes SET atualizado_em = now() WHERE id = $1`, avaliacaoID); err != nil {
			return err
		}
		if len(notasPorCriterio) == len(criterios) {
			var valores []notas.Centesimos
			for _, v := range notasPorCriterio {
				valores = append(valores, v)
			}
			if previa, err := notas.NotaAvaliador(valores); err == nil {
				resposta["notaPrevia"] = previa.FormatarBR()
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.fila.Notificar()
	return responderOK(w, resposta)
}

// PUT /docente/avaliacoes/{trabalhoId}/parecer {parecer}
// O parecer é apenas informativo (subsídio para o aluno corrigir o artigo): não entra na nota,
// pode ser salvo antes dela e continua editável após o envio da nota, até a geração da ATA.
func (s *Servidor) docenteSalvarParecer(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	ctx := r.Context()
	trabalhoID := chi.URLParam(r, "trabalhoId")
	var req struct {
		Parecer string `json:"parecer"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	req.Parecer = strings.TrimSpace(req.Parecer)
	if utf8.RuneCountInString(req.Parecer) > 20000 {
		return erroCampo("parecer", "O parecer deve ter no máximo 20.000 caracteres.")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SELECT id FROM trabalhos WHERE id = $1 FOR UPDATE`, trabalhoID); err != nil {
		return err
	}
	if err := exigirMembroBanca(ctx, tx, trabalhoID, ses.UsuarioID); err != nil {
		return err
	}
	var possuiATA bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM atas WHERE trabalho_id = $1 AND estado <> 'cancelada')`, trabalhoID).Scan(&possuiATA); err != nil {
		return err
	}
	if possuiATA {
		return erroConflito("A ATA deste trabalho já foi gerada; o parecer não pode mais ser alterado.")
	}
	var atualizadoEm string
	if err := tx.QueryRow(ctx, `INSERT INTO avaliacoes (trabalho_id, avaliador_id, parecer, parecer_atualizado_em) VALUES ($1, $2, $3, now())
		ON CONFLICT (trabalho_id, avaliador_id) DO UPDATE SET parecer = $3, parecer_atualizado_em = now(), atualizado_em = now()
		RETURNING to_char(parecer_atualizado_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI')`,
		trabalhoID, ses.UsuarioID, req.Parecer).Scan(&atualizadoEm); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return responderOK(w, map[string]string{"parecerAtualizadoEm": atualizadoEm})
}

// POST /docente/trabalhos/{id}/avaliacoes/{avaliadorId}/reabrir {motivo} — procedimento autorizado (orientador) e auditado.
func (s *Servidor) docenteReabrirAvaliacao(w http.ResponseWriter, r *http.Request) error {
	id, avaliador := chi.URLParam(r, "id"), chi.URLParam(r, "avaliadorId")
	if _, err := s.exigirOrientador(r, id); err != nil {
		return err
	}
	var req struct {
		Motivo string `json:"motivo"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	if len(strings.TrimSpace(req.Motivo)) < 5 {
		return erroCampo("motivo", "Informe o motivo da reabertura.")
	}
	ses := sessaoDe(r)
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var possuiATA bool
	if err := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM atas WHERE trabalho_id = $1 AND estado <> 'cancelada')`, id).Scan(&possuiATA); err != nil {
		return err
	}
	if possuiATA {
		return erroConflito("Cancele/reinicie a ATA antes de reabrir uma avaliação.")
	}
	var notaAnterior *string
	err = tx.QueryRow(r.Context(), `UPDATE avaliacoes SET estado = 'rascunho', enviada_em = NULL, atualizado_em = now()
		WHERE trabalho_id = $1 AND avaliador_id = $2 AND estado = 'finalizada'
		RETURNING (SELECT nota_calculada::text FROM avaliacoes WHERE trabalho_id = $1 AND avaliador_id = $2)`, id, avaliador).Scan(&notaAnterior)
	if errors.Is(err, pgx.ErrNoRows) {
		return erroNaoEncontrado("Avaliação finalizada não encontrada.")
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(r.Context(), `UPDATE avaliacoes SET nota_calculada = NULL WHERE trabalho_id = $1 AND avaliador_id = $2`, id, avaliador); err != nil {
		return err
	}
	if _, err := regras.RecalcularEstadoTrabalho(r.Context(), tx, id, ses.UsuarioID, "Avaliação reaberta: "+req.Motivo); err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "reabrir_avaliacao", "trabalho", id,
		map[string]any{"avaliador": avaliador, "motivo": req.Motivo, "notaAnterior": notaAnterior}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}
