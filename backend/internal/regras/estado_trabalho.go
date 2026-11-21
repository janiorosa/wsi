// Pacote regras concentra regras de negócio compartilhadas entre a API e a fila.
package regras

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"wsi/internal/banco"
)

// Estados do trabalho (seção 8.3).
const (
	EstadoRascunho              = "rascunho"
	EstadoSubmetido             = "submetido"
	EstadoAguardandoAvaliadores = "aguardando_avaliadores"
	EstadoAguardandoAvaliacoes  = "aguardando_avaliacoes"
	EstadoAvaliacoesConcluidas  = "avaliacoes_concluidas"
	EstadoATAAguardandoGeracao  = "ata_aguardando_geracao"
	EstadoATAGerada             = "ata_gerada"
	EstadoAssinaturas           = "assinaturas_em_andamento"
	EstadoATAFinalizada         = "ata_finalizada"
	EstadoDocumentacaoPendente  = "documentacao_pendente"
	EstadoConcluido             = "concluido"
	EstadoCancelado             = "cancelado"
)

// RecalcularEstadoTrabalho deriva o estado do trabalho a partir dos dados
// (submissão, banca, avaliações, ATA e declaração) e registra o histórico quando
// houver mudança. As transições são sempre controladas por esta função no backend.
func RecalcularEstadoTrabalho(ctx context.Context, q banco.Executor, trabalhoID, atorID, observacao string) (string, error) {
	var atual, titulo, resumo, alunoID string
	var temEvento bool
	err := q.QueryRow(ctx, `SELECT estado, titulo, resumo, aluno_id::text, evento_id IS NOT NULL
		FROM trabalhos WHERE id = $1 FOR UPDATE`, trabalhoID).Scan(&atual, &titulo, &resumo, &alunoID, &temEvento)
	if err != nil {
		return "", err
	}
	if atual == EstadoCancelado {
		return atual, nil
	}
	novo, err := calcular(ctx, q, trabalhoID, alunoID, titulo, resumo, temEvento)
	if err != nil {
		return "", err
	}
	if novo == atual {
		return atual, nil
	}
	var ator any
	if atorID != "" {
		ator = atorID
	}
	if _, err := q.Exec(ctx, `UPDATE trabalhos SET estado = $2, atualizado_em = now(),
		submetido_em = CASE WHEN submetido_em IS NULL AND $2 <> 'rascunho' THEN now() ELSE submetido_em END
		WHERE id = $1`, trabalhoID, novo); err != nil {
		return "", err
	}
	_, err = q.Exec(ctx, `INSERT INTO historico_estados_trabalho (trabalho_id, estado_anterior, estado_novo, ator_id, observacao)
		VALUES ($1, $2, $3, $4, $5)`, trabalhoID, atual, novo, ator, observacao)
	return novo, err
}

func calcular(ctx context.Context, q banco.Executor, trabalhoID, alunoID, titulo, resumo string, temEvento bool) (string, error) {
	var estadoATA string
	var posicao int
	err := q.QueryRow(ctx, `SELECT estado, posicao_atual FROM atas WHERE trabalho_id = $1 AND estado <> 'cancelada'`, trabalhoID).
		Scan(&estadoATA, &posicao)
	if err == nil {
		switch estadoATA {
		case "gerando":
			return EstadoATAAguardandoGeracao, nil
		case "aguardando_assinaturas":
			if posicao <= 1 {
				return EstadoATAGerada, nil
			}
			return EstadoAssinaturas, nil
		case "finalizada":
			var declaracao bool
			if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM declaracoes_assinadas WHERE aluno_id = $1)`, alunoID).Scan(&declaracao); err != nil {
				return "", err
			}
			if declaracao {
				return EstadoConcluido, nil
			}
			return EstadoDocumentacaoPendente, nil
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	if strings.TrimSpace(titulo) == "" || strings.TrimSpace(resumo) == "" {
		return EstadoRascunho, nil
	}
	var examinadores, pendentes int
	err = q.QueryRow(ctx, `
		SELECT
		  count(*) FILTER (WHERE mb.papel = 'examinador'),
		  count(*) FILTER (WHERE mb.obrigatorio AND (a.estado IS DISTINCT FROM 'finalizada'))
		FROM membros_banca mb
		LEFT JOIN avaliacoes a ON a.trabalho_id = mb.trabalho_id AND a.avaliador_id = mb.docente_id
		WHERE mb.trabalho_id = $1 AND mb.situacao = 'ativo'`, trabalhoID).Scan(&examinadores, &pendentes)
	if err != nil {
		return "", err
	}
	switch {
	case examinadores == 0 && !temEvento:
		return EstadoSubmetido, nil
	case examinadores == 0:
		return EstadoAguardandoAvaliadores, nil
	case pendentes > 0:
		return EstadoAguardandoAvaliacoes, nil
	default:
		return EstadoAvaliacoesConcluidas, nil
	}
}

// RecalcularTrabalhosDoAluno recalcula os trabalhos ativos de um aluno (ex.: após enviar a declaração).
func RecalcularTrabalhosDoAluno(ctx context.Context, q banco.Executor, alunoID, atorID, observacao string) error {
	rows, err := q.Query(ctx, `SELECT id::text FROM trabalhos WHERE aluno_id = $1 AND estado <> 'cancelado'`, alunoID)
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
		if _, err := RecalcularEstadoTrabalho(ctx, q, id, atorID, observacao); err != nil {
			return err
		}
	}
	return nil
}
