// Pacote fila implementa a fila assíncrona de documentos persistida no PostgreSQL.
//
// - Estados: pendente, processando, concluida, falhou.
// - Duplo clique: índice único parcial (tipo, entidade_id) para tarefas ativas.
// - Concorrência: FOR UPDATE SKIP LOCKED impede dois trabalhadores na mesma tarefa.
// - Reinício: tarefas "processando" voltam a "pendente" na inicialização.
package fila

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"wsi/internal/banco"
)

const (
	TipoDeclaracao    = "gerar_declaracao"
	TipoATA           = "gerar_ata"
	TipoApresentacoes = "gerar_apresentacoes"
	TipoAnais         = "gerar_anais"
)

type Tarefa struct {
	ID            string
	Tipo          string
	EntidadeID    string
	Parametros    map[string]string
	SolicitadoPor string
	Tentativas    int
}

// Processador executa uma tarefa. Deve devolver o identificador do resultado (documento, ATA, publicação).
type Processador func(ctx context.Context, t Tarefa, progresso func(int)) (resultadoID string, avisos []string, err error)

type Fila struct {
	pool          *pgxpool.Pool
	concorrencia  int
	processadores map[string]Processador
	aviso         chan struct{}
	espera        sync.WaitGroup
}

func Nova(pool *pgxpool.Pool, concorrencia int) *Fila {
	return &Fila{pool: pool, concorrencia: concorrencia, processadores: map[string]Processador{}, aviso: make(chan struct{}, 1)}
}

func (f *Fila) Registrar(tipo string, p Processador) { f.processadores[tipo] = p }

// Notificar acorda um trabalhador (não bloqueante).
func (f *Fila) Notificar() {
	select {
	case f.aviso <- struct{}{}:
	default:
	}
}

// Enfileirar registra uma tarefa. Se já existir tarefa ativa para o mesmo tipo e
// entidade, devolve a existente (proteção contra duplo clique).
func Enfileirar(ctx context.Context, q banco.Executor, tipo, entidadeID string, parametros map[string]string, solicitante string) (string, error) {
	if parametros == nil {
		parametros = map[string]string{}
	}
	p, err := json.Marshal(parametros)
	if err != nil {
		return "", err
	}
	var id string
	err = q.QueryRow(ctx, `
		INSERT INTO tarefas_documentos (tipo, entidade_id, parametros, solicitado_por)
		VALUES ($1, $2, $3::jsonb, $4)
		ON CONFLICT (tipo, entidade_id) WHERE estado IN ('pendente', 'processando') DO NOTHING
		RETURNING id::text`, tipo, entidadeID, string(p), solicitante).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = q.QueryRow(ctx, `SELECT id::text FROM tarefas_documentos
			WHERE tipo = $1 AND entidade_id = $2 AND estado IN ('pendente', 'processando')`, tipo, entidadeID).Scan(&id)
	}
	return id, err
}

// Iniciar recupera tarefas interrompidas e inicia os trabalhadores.
func (f *Fila) Iniciar(ctx context.Context) error {
	tag, err := f.pool.Exec(ctx, `UPDATE tarefas_documentos SET estado = 'pendente', progresso = 0 WHERE estado = 'processando'`)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		slog.Info("tarefas interrompidas recolocadas na fila", "quantidade", tag.RowsAffected())
	}
	for i := 0; i < f.concorrencia; i++ {
		f.espera.Add(1)
		go f.trabalhador(ctx, i+1)
	}
	return nil
}

// Aguardar espera os trabalhadores terminarem após o cancelamento do contexto.
func (f *Fila) Aguardar() { f.espera.Wait() }

func (f *Fila) trabalhador(ctx context.Context, numero int) {
	defer f.espera.Done()
	for {
		if ctx.Err() != nil {
			return
		}
		processou, err := f.processarProxima(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Error("erro na fila de documentos", "trabalhador", numero, "erro", err)
		}
		if processou {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-f.aviso:
		case <-time.After(3 * time.Second):
		}
	}
}

func (f *Fila) processarProxima(ctx context.Context) (bool, error) {
	var t Tarefa
	var parametros []byte
	err := f.pool.QueryRow(ctx, `
		UPDATE tarefas_documentos
		SET estado = 'processando', iniciado_em = now(), concluido_em = NULL,
		    tentativas = tentativas + 1, progresso = 5, erro = '', avisos = ''
		WHERE id = (
			SELECT id FROM tarefas_documentos
			WHERE estado = 'pendente'
			ORDER BY criado_em
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id::text, tipo, entidade_id::text, parametros::text, solicitado_por::text, tentativas`).
		Scan(&t.ID, &t.Tipo, &t.EntidadeID, &parametros, &t.SolicitadoPor, &t.Tentativas)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_ = json.Unmarshal(parametros, &t.Parametros)

	inicio := time.Now()
	resultado, avisos, errProc := f.executar(ctx, t)
	if ctx.Err() != nil {
		// Desligamento: a tarefa volta a "pendente" no próximo início.
		return true, nil
	}
	if errProc != nil {
		slog.Warn("tarefa de documento falhou", "tarefa", t.ID, "tipo", t.Tipo, "erro", errProc)
		_, err = f.pool.Exec(context.Background(), `UPDATE tarefas_documentos
			SET estado = 'falhou', erro = $2, concluido_em = now() WHERE id = $1`, t.ID, limitar(errProc.Error(), 2000))
		return true, err
	}
	var resultadoID any
	if resultado != "" {
		resultadoID = resultado
	}
	avisosTexto := ""
	for i, a := range avisos {
		if i > 0 {
			avisosTexto += "; "
		}
		avisosTexto += a
	}
	_, err = f.pool.Exec(context.Background(), `UPDATE tarefas_documentos
		SET estado = 'concluida', progresso = 100, resultado_id = $2, avisos = $3, concluido_em = now() WHERE id = $1`,
		t.ID, resultadoID, limitar(avisosTexto, 2000))
	slog.Info("tarefa de documento concluída", "tarefa", t.ID, "tipo", t.Tipo, "duracao", time.Since(inicio).String())
	return true, err
}

func (f *Fila) executar(ctx context.Context, t Tarefa) (resultado string, avisos []string, err error) {
	p, ok := f.processadores[t.Tipo]
	if !ok {
		return "", nil, fmt.Errorf("tipo de tarefa desconhecido: %s", t.Tipo)
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Error("pânico ao processar tarefa", "tarefa", t.ID, "panico", r, "pilha", string(debug.Stack()))
			err = fmt.Errorf("erro interno ao gerar o documento")
		}
	}()
	progresso := func(v int) {
		if v < 0 || v > 99 {
			return
		}
		f.pool.Exec(context.Background(), `UPDATE tarefas_documentos SET progresso = $2 WHERE id = $1`, t.ID, v)
	}
	return p(ctx, t, progresso)
}

func limitar(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
