package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"wsi/internal/banco"
	"wsi/internal/regras"
)

func possuiPerfil(ctx context.Context, q banco.Executor, usuarioID, perfil string) (bool, error) {
	var existe bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM perfis_usuario pu JOIN usuarios u ON u.id = pu.usuario_id
		WHERE pu.usuario_id = $1 AND pu.perfil = $2 AND u.situacao <> 'bloqueado')`, usuarioID, perfil).Scan(&existe)
	return existe, err
}

// criarVinculo cria o vínculo aluno–docente. Se o aluno já tiver orientador:
// sem permitirTroca → conflito (não substituir silenciosamente); com permitirTroca → encerra o anterior.
func criarVinculo(ctx context.Context, tx pgx.Tx, alunoID, docenteID, atorID string, permitirTroca bool) (string, error) {
	if ok, err := possuiPerfil(ctx, tx, alunoID, PerfilAluno); err != nil {
		return "", err
	} else if !ok {
		return "", erroRequisicao("O usuário informado não é um aluno.")
	}
	if ok, err := possuiPerfil(ctx, tx, docenteID, PerfilDocente); err != nil {
		return "", err
	} else if !ok {
		return "", erroRequisicao("O orientador informado não é um docente aprovado.")
	}
	// Bloqueia o aluno para evitar vínculos concorrentes.
	if _, err := tx.Exec(ctx, `SELECT id FROM usuarios WHERE id = $1 FOR UPDATE`, alunoID); err != nil {
		return "", err
	}
	var vinculoAtual, docenteAtual string
	err := tx.QueryRow(ctx, `SELECT id::text, docente_id::text FROM vinculos_orientacao WHERE aluno_id = $1 AND situacao = 'ativo'`, alunoID).
		Scan(&vinculoAtual, &docenteAtual)
	switch {
	case err == nil && docenteAtual == docenteID:
		return "", erroConflito("Este docente já é o orientador do aluno.")
	case err == nil && !permitirTroca:
		return "", &ErroAPI{Status: http.StatusConflict, Codigo: "aluno_com_orientador",
			Mensagem: "O aluno já possui orientador. A troca deve ser confirmada explicitamente."}
	case err == nil:
		if _, err := tx.Exec(ctx, `UPDATE vinculos_orientacao SET situacao = 'encerrado', fim = now(), encerrado_por = $2,
			motivo_encerramento = 'Troca de orientador' WHERE id = $1`, vinculoAtual, atorID); err != nil {
			return "", err
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return "", err
	}
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO vinculos_orientacao (aluno_id, docente_id, criado_por) VALUES ($1, $2, $3) RETURNING id::text`,
		alunoID, docenteID, atorID).Scan(&id); err != nil {
		return "", err
	}
	if err := sincronizarOrientadorTrabalhos(ctx, tx, alunoID, docenteID, atorID); err != nil {
		return "", err
	}
	return id, nil
}

// encerrarVinculo encerra sem apagar histórico nem trabalhos.
func encerrarVinculo(ctx context.Context, tx pgx.Tx, vinculoID, atorID, motivo string) (alunoID string, err error) {
	err = tx.QueryRow(ctx, `UPDATE vinculos_orientacao SET situacao = 'encerrado', fim = now(), encerrado_por = $2, motivo_encerramento = $3
		WHERE id = $1 AND situacao = 'ativo' RETURNING aluno_id::text`, vinculoID, atorID, motivo).Scan(&alunoID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", erroNaoEncontrado("Vínculo ativo não encontrado.")
	}
	if err != nil {
		return "", err
	}
	return alunoID, sincronizarOrientadorTrabalhos(ctx, tx, alunoID, "", atorID)
}

// sincronizarOrientadorTrabalhos atualiza o orientador dos trabalhos em andamento que ainda não têm ATA ativa.
// O orientador também integra a banca (papel "orientador", ordem 0).
func sincronizarOrientadorTrabalhos(ctx context.Context, tx pgx.Tx, alunoID, docenteID, atorID string) error {
	rows, err := tx.Query(ctx, `SELECT t.id::text FROM trabalhos t
		WHERE t.aluno_id = $1 AND t.estado NOT IN ('cancelado', 'concluido')
		  AND NOT EXISTS (SELECT 1 FROM atas a WHERE a.trabalho_id = t.id AND a.estado <> 'cancelada')`, alunoID)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.Exec(ctx, `UPDATE trabalhos SET orientador_id = $2, atualizado_em = now() WHERE id = $1`, id, textoOuNil(docenteID)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE membros_banca SET situacao = 'removido' WHERE trabalho_id = $1 AND papel = 'orientador' AND situacao = 'ativo'`, id); err != nil {
			return err
		}
		if docenteID != "" {
			// Se o novo orientador era examinador deste trabalho, deixa de ser examinador.
			if _, err := tx.Exec(ctx, `UPDATE membros_banca SET situacao = 'removido' WHERE trabalho_id = $1 AND docente_id = $2 AND situacao = 'ativo'`, id, docenteID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO membros_banca (trabalho_id, docente_id, papel, ordem, obrigatorio, designado_por)
				VALUES ($1, $2, 'orientador', 0, true, $3)`, id, docenteID, atorID); err != nil {
				return err
			}
		}
		if _, err := regras.RecalcularEstadoTrabalho(ctx, tx, id, atorID, "Orientação alterada"); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------
// Vínculos pelo ADMIN
// ---------------------------------------------------------------------

type itemVinculo struct {
	ID          string  `json:"id"`
	AlunoID     string  `json:"alunoId"`
	AlunoNome   string  `json:"alunoNome"`
	DocenteID   string  `json:"docenteId"`
	DocenteNome string  `json:"docenteNome"`
	Situacao    string  `json:"situacao"`
	Inicio      string  `json:"inicio"`
	Fim         *string `json:"fim"`
	CriadoPor   string  `json:"criadoPor"`
	Motivo      string  `json:"motivoEncerramento"`
}

func (s *Servidor) adminListarVinculos(w http.ResponseWriter, r *http.Request) error {
	usuario := r.URL.Query().Get("usuarioId")
	rows, err := s.pool.Query(r.Context(), `SELECT v.id::text, a.id::text, a.nome_completo, d.id::text, d.nome_completo, v.situacao,
		to_char(v.inicio AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI'),
		to_char(v.fim AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI'), c.nome_completo, v.motivo_encerramento
		FROM vinculos_orientacao v JOIN usuarios a ON a.id = v.aluno_id JOIN usuarios d ON d.id = v.docente_id
		JOIN usuarios c ON c.id = v.criado_por
		WHERE ($1 = '' OR v.aluno_id::text = $1 OR v.docente_id::text = $1)
		ORDER BY v.situacao, v.inicio DESC LIMIT 200`, usuario)
	if err != nil {
		return err
	}
	defer rows.Close()
	itens := []itemVinculo{}
	for rows.Next() {
		var v itemVinculo
		if err := rows.Scan(&v.ID, &v.AlunoID, &v.AlunoNome, &v.DocenteID, &v.DocenteNome, &v.Situacao, &v.Inicio, &v.Fim, &v.CriadoPor, &v.Motivo); err != nil {
			return err
		}
		itens = append(itens, v)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return responderOK(w, itens)
}

func (s *Servidor) adminCriarVinculo(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		AlunoID        string `json:"alunoId"`
		DocenteID      string `json:"docenteId"`
		ConfirmarTroca bool   `json:"confirmarTroca"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	ses := sessaoDe(r)
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	id, err := criarVinculo(r.Context(), tx, req.AlunoID, req.DocenteID, ses.UsuarioID, req.ConfirmarTroca)
	if err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "criar_vinculo_orientacao", "vinculo_orientacao", id,
		map[string]any{"aluno": req.AlunoID, "docente": req.DocenteID, "troca": req.ConfirmarTroca}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	responderJSON(w, http.StatusCreated, map[string]string{"id": id})
	return nil
}

func (s *Servidor) adminEncerrarVinculo(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Motivo string `json:"motivo"`
	}
	lerJSON(r, &req)
	ses := sessaoDe(r)
	id := chi.URLParam(r, "id")
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err := encerrarVinculo(r.Context(), tx, id, ses.UsuarioID, req.Motivo); err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "encerrar_vinculo_orientacao", "vinculo_orientacao", id, map[string]any{"motivo": req.Motivo}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}
