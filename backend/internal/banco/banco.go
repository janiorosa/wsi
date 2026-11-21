// Pacote banco abre o pool de conexões e aplica as migrações versionadas.
package banco

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migracoes/*.sql
var arquivosMigracao embed.FS

var regexMigracao = regexp.MustCompile(`^[0-9]{4}_[a-z0-9_]+$`)

func Conectar(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("URL do banco inválida: %w", err)
	}
	cfg.MaxConns = 10
	// Protocolo simples: parâmetros enviados como literais escapados pelo pgx.
	// Evita ambiguidades de tipo (uuid, date, time, numeric) com strings Go.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	var pool *pgxpool.Pool
	// O PostgreSQL pode demorar alguns segundos para aceitar conexões ao subir o compose.
	for tentativa := 1; tentativa <= 30; tentativa++ {
		pool, err = pgxpool.NewWithConfig(ctx, cfg)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				return pool, nil
			}
			pool.Close()
		}
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("não foi possível conectar ao banco: %w", err)
}

// Migrar aplica, em ordem, os arquivos NNNN_nome.sql ainda não aplicados.
// Cada migração roda em sua própria transação e é registrada em migracoes_aplicadas.
func Migrar(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS migracoes_aplicadas (
		versao text PRIMARY KEY,
		aplicada_em timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	// Bloqueio consultivo: impede duas instâncias migrando ao mesmo tempo.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(727001)`); err != nil {
		return err
	}
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock(727001)`)

	nomes, err := fs.Glob(arquivosMigracao, "migracoes/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(nomes)
	for _, nome := range nomes {
		versao := strings.TrimSuffix(strings.TrimPrefix(nome, "migracoes/"), ".sql")
		// Somente "NNNN_nome.sql": ignora arquivos ocultos (ex.: "._0001..." criados pelo macOS).
		if !regexMigracao.MatchString(versao) {
			continue
		}
		var existe bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM migracoes_aplicadas WHERE versao = $1)`, versao).Scan(&existe); err != nil {
			return err
		}
		if existe {
			continue
		}
		conteudo, err := arquivosMigracao.ReadFile(nome)
		if err != nil {
			return err
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(conteudo)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("migração %s falhou: %w", versao, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO migracoes_aplicadas (versao) VALUES ($1)`, versao); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Executor é satisfeito tanto pelo pool quanto por transações (pgx.Tx).
type Executor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
