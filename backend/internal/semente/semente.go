// Pacote semente aplica a carga inicial idempotente: dados institucionais,
// docentes informados pelo responsável e o ADMIN inicial.
// Não sobrescreve informações alteradas depois. Não inventa SIAPEs nem critérios.
package semente

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"wsi/internal/documentos"
)

type docenteInicial struct {
	Nome  string
	Email string
	SIAPE string // vazio = pendente de conferência
}

// Docentes informados na especificação (seção 21).
var docentesIniciais = []docenteInicial{
	{"Angelo Magno de Jesus", "angelo.jesus@ifmg.edu.br", ""},
	{"Charles Tim Batista Garrocho", "charles.garrocho@ifmg.edu.br", "1146652"},
	{"Daniela Costa Terra", "daniela.terra@ifmg.edu.br", ""},
	{"Edilus de Carvalho Castro Penido", "edilus.penido@ifmg.edu.br", ""},
	{"Janio Rosa da Silva", "janio.silva@ifmg.edu.br", "1752938"},
	{"Luciana Lourdes Silva", "luciana.lourdes.silva@ifmg.edu.br", ""},
	{"Suelen Mapa de Paula", "suelen.mapa@ifmg.edu.br", ""},
	{"Ederson Naves Fernandes Gonçalves Júnior", "ederson.junior@ifmg.edu.br", ""},
}

func Aplicar(ctx context.Context, pool *pgxpool.Pool, emailAdmin string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())

	for chave, valor := range map[string]string{
		documentos.ChaveCabecalho: documentos.CabecalhoPadraoJSON(),
		documentos.ChaveCidade:    "Ouro Branco",
	} {
		if _, err := tx.Exec(ctx, `INSERT INTO configuracoes_institucionais (chave, valor) VALUES ($1, $2) ON CONFLICT (chave) DO NOTHING`, chave, valor); err != nil {
			return err
		}
	}

	// Critério geral fixo "Nota" (0 a 100): sempre presente e ativo.
	if _, err := tx.Exec(ctx, `INSERT INTO criterios_avaliacao (nome, descricao, nota_minima, nota_maxima, ativo, ordem, fixo)
		SELECT 'Nota', 'Nota atribuída ao trabalho pelo docente avaliador, de 0 a 100.', 0, 100, true, 0, true
		WHERE NOT EXISTS (SELECT 1 FROM criterios_avaliacao WHERE fixo)`); err != nil {
		return err
	}

	for _, d := range docentesIniciais {
		var id string
		err := tx.QueryRow(ctx, `INSERT INTO usuarios (nome_completo, email, identificador_institucional, tipo_identificador, situacao, dados_pendentes_conferencia)
			VALUES ($1, $2, $3, 'SIAPE', 'aguardando_primeiro_acesso', $4)
			ON CONFLICT (email) DO NOTHING RETURNING id::text`, d.Nome, d.Email, d.SIAPE, d.SIAPE == "").Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			continue // já existe: não sobrescrever
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO perfis_docente (usuario_id, siape, situacao_aprovacao, aprovado_em) VALUES ($1, $2, 'aprovado', now())`, id, d.SIAPE); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO perfis_usuario (usuario_id, perfil) VALUES ($1, 'DOCENTE')`, id); err != nil {
			return err
		}
		slog.Info("docente inicial cadastrado", "email", d.Email, "siape_pendente", d.SIAPE == "")
	}

	// ADMIN inicial (somente se ainda não existir nenhum administrador).
	var totalAdmins int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM perfis_usuario WHERE perfil = 'ADMIN'`).Scan(&totalAdmins); err != nil {
		return err
	}
	var adminID string
	var temSenha bool
	err = tx.QueryRow(ctx, `SELECT id::text, hash_senha IS NOT NULL FROM usuarios WHERE email = $1`, strings.ToLower(emailAdmin)).Scan(&adminID, &temSenha)
	if errors.Is(err, pgx.ErrNoRows) {
		slog.Warn("e-mail do ADMIN inicial não encontrado entre os usuários", "email", emailAdmin)
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if totalAdmins == 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO perfis_usuario (usuario_id, perfil) VALUES ($1, 'ADMIN') ON CONFLICT DO NOTHING`, adminID); err != nil {
			return err
		}
		slog.Info("perfil ADMIN atribuído ao administrador inicial", "email", emailAdmin)
	}

	// Primeiro acesso do ADMIN: pela tela "Primeiro acesso" (nome completo, e-mail e SIAPE).
	return tx.Commit(ctx)
}
