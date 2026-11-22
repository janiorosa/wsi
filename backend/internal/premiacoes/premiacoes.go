// Pacote premiacoes calcula o resultado das premiações de um evento a partir dos
// votos dos docentes (cada docente vota em até dois trabalhos por premiação).
package premiacoes

import (
	"context"

	"wsi/internal/banco"
)

// NomePadrao é a premiação criada automaticamente em todo evento.
const NomePadrao = "Prêmio Destaque: Reconhecendo a Excelência Acadêmica"

// MaximoVotosPorDocente é o número máximo de trabalhos em que cada docente vota, por premiação.
const MaximoVotosPorDocente = 2

type Trabalho struct {
	ID         string `json:"trabalhoId"`
	Titulo     string `json:"titulo"`
	Aluno      string `json:"aluno"`
	Orientador string `json:"orientador"`
	Votos      int    `json:"votos"`
}

type Resultado struct {
	ID         string     `json:"id"`
	Nome       string     `json:"nome"`
	Descricao  string     `json:"descricao"`
	Ordem      int        `json:"ordem"`
	Votantes   int        `json:"votantes"`
	Ranking    []Trabalho `json:"ranking"`    // somente trabalhos com voto, do mais votado ao menos votado
	Vencedores []Trabalho `json:"vencedores"` // mais votado(s); empate: todos
}

// CriarPadrao cria a premiação padrão do evento (idempotente).
func CriarPadrao(ctx context.Context, q banco.Executor, eventoID string, criadoPor *string) error {
	_, err := q.Exec(ctx, `INSERT INTO premiacoes (evento_id, nome, ordem, criado_por) VALUES ($1, $2, 1, $3)
		ON CONFLICT (evento_id, nome) DO NOTHING`, eventoID, NomePadrao, criadoPor)
	return err
}

// Resultados devolve as premiações do evento com a contagem de votos.
// Só contam votos em trabalhos ainda associados ao evento e não cancelados.
func Resultados(ctx context.Context, q banco.Executor, eventoID string) ([]Resultado, error) {
	rows, err := q.Query(ctx, `SELECT p.id::text, p.nome, p.descricao, p.ordem,
		(SELECT count(DISTINCT v.docente_id) FROM votos_premiacao v WHERE v.premiacao_id = p.id)
		FROM premiacoes p WHERE p.evento_id = $1 ORDER BY p.ordem, p.nome`, eventoID)
	if err != nil {
		return nil, err
	}
	itens := []Resultado{}
	for rows.Next() {
		var r Resultado
		if err := rows.Scan(&r.ID, &r.Nome, &r.Descricao, &r.Ordem, &r.Votantes); err != nil {
			rows.Close()
			return nil, err
		}
		r.Ranking, r.Vencedores = []Trabalho{}, []Trabalho{}
		itens = append(itens, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range itens {
		r, err := q.Query(ctx, `SELECT t.id::text, t.titulo, a.nome_completo, COALESCE(o.nome_completo, ''), count(*)
			FROM votos_premiacao v JOIN trabalhos t ON t.id = v.trabalho_id
			JOIN usuarios a ON a.id = t.aluno_id LEFT JOIN usuarios o ON o.id = t.orientador_id
			WHERE v.premiacao_id = $1 AND t.evento_id = $2 AND t.estado <> 'cancelado'
			GROUP BY t.id, t.titulo, a.nome_completo, o.nome_completo
			ORDER BY count(*) DESC, t.titulo, t.id`, itens[i].ID, eventoID)
		if err != nil {
			return nil, err
		}
		for r.Next() {
			var t Trabalho
			if err := r.Scan(&t.ID, &t.Titulo, &t.Aluno, &t.Orientador, &t.Votos); err != nil {
				r.Close()
				return nil, err
			}
			itens[i].Ranking = append(itens[i].Ranking, t)
		}
		r.Close()
		if err := r.Err(); err != nil {
			return nil, err
		}
		if len(itens[i].Ranking) > 0 {
			maior := itens[i].Ranking[0].Votos
			for _, t := range itens[i].Ranking {
				if t.Votos == maior {
					itens[i].Vencedores = append(itens[i].Vencedores, t)
				}
			}
		}
	}
	return itens, nil
}
