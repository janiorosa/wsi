package api

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"wsi/internal/banco"
)

type arquivoTrabalho struct {
	ID           string `json:"id"`
	Tipo         string `json:"tipo"`
	NomeOriginal string `json:"nomeOriginal"`
	Versao       int    `json:"versao"`
	Tamanho      int64  `json:"tamanho"`
	SHA256       string `json:"sha256"`
	EnviadoEm    string `json:"enviadoEm"`
	EnviadoPor   string `json:"enviadoPor"`
}

// listarArquivos devolve todas as versões, da mais recente para a mais antiga, agrupadas por tipo.
func listarArquivos(ctx context.Context, q banco.Executor, trabalhoID string) (map[string][]arquivoTrabalho, error) {
	rows, err := q.Query(ctx, `SELECT af.id::text, af.tipo, af.nome_original, af.versao, af.tamanho, af.sha256,
		to_char(af.enviado_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI'), u.nome_completo
		FROM arquivos_trabalho af JOIN usuarios u ON u.id = af.enviado_por
		WHERE af.trabalho_id = $1 ORDER BY af.tipo, af.versao DESC`, trabalhoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	r := map[string][]arquivoTrabalho{"artigo": {}, "apresentacao": {}}
	for rows.Next() {
		var a arquivoTrabalho
		if err := rows.Scan(&a.ID, &a.Tipo, &a.NomeOriginal, &a.Versao, &a.Tamanho, &a.SHA256, &a.EnviadoEm, &a.EnviadoPor); err != nil {
			return nil, err
		}
		r[a.Tipo] = append(r[a.Tipo], a)
	}
	return r, rows.Err()
}

type eventoTrabalho struct {
	ID               string  `json:"id"`
	Nome             string  `json:"nome"`
	Data             string  `json:"data"`
	Horario          string  `json:"horario"`
	Local            string  `json:"local"`
	SessaoID         *string `json:"sessaoId"`
	Sessao           *string `json:"sessao"`
	Futuro           bool    `json:"futuro"`
}

// eventoDoTrabalho: data/horário/local efetivos (sessão, se houver; senão evento).
func eventoDoTrabalho(ctx context.Context, q banco.Executor, trabalhoID string) (*eventoTrabalho, error) {
	var e eventoTrabalho
	err := q.QueryRow(ctx, `SELECT e.id::text, e.nome, to_char(COALESCE(se.data, e.data_apresentacao), 'DD/MM/YYYY'),
		to_char(COALESCE(se.horario, e.horario_apresentacao), 'HH24:MI'), COALESCE(se.local, e.local), se.id::text, se.titulo,
		(COALESCE(se.data, e.data_apresentacao) + COALESCE(se.horario, e.horario_apresentacao)) > (now() AT TIME ZONE 'America/Sao_Paulo')
		FROM trabalhos t JOIN eventos e ON e.id = t.evento_id LEFT JOIN sessoes_evento se ON se.id = t.sessao_id
		WHERE t.id = $1`, trabalhoID).Scan(&e.ID, &e.Nome, &e.Data, &e.Horario, &e.Local, &e.SessaoID, &e.Sessao, &e.Futuro)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &e, err
}

type signatarioResumo struct {
	Ordem     int     `json:"ordem"`
	Rotulo    string  `json:"rotulo"`
	Nome      string  `json:"nome"`
	Estado    string  `json:"estado"`
	EnviadoEm *string `json:"enviadoEm"`
	SouEu     bool    `json:"souEu"`
}

type ataResumo struct {
	ID            string             `json:"id"`
	Versao        int                `json:"versao"`
	Estado        string             `json:"estado"`
	PosicaoAtual  int                `json:"posicaoAtual"`
	MinhaVez      bool               `json:"minhaVez"`
	SouSignatario bool               `json:"souSignatario"`
	Situacao      string             `json:"situacao"`
	ErroGeracao   string             `json:"erroGeracao"`
	GeradaEm      *string            `json:"geradaEm"`
	Signatarios   []signatarioResumo `json:"signatarios"`
	PodeBaixar    bool               `json:"podeBaixar"`
	// Docentes da banca: baixam a qualquer momento a ATA original (em branco) e a versão
	// com as assinaturas já coletadas, mesmo que ainda faltem assinaturas.
	PodeBaixarOriginal   bool `json:"podeBaixarOriginal"`
	PodeBaixarAssinada   bool `json:"podeBaixarAssinada"`
	AssinaturasColetadas int  `json:"assinaturasColetadas"`
}

// resumoATA descreve a ATA ativa do trabalho do ponto de vista do usuário informado.
// Nunca inclui a nota: ela consta somente no PDF, acessível aos participantes.
func resumoATA(ctx context.Context, q banco.Executor, trabalhoID, usuarioID string) (*ataResumo, error) {
	var a ataResumo
	err := q.QueryRow(ctx, `SELECT id::text, versao, estado, posicao_atual, erro_geracao,
		to_char(gerado_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI')
		FROM atas WHERE trabalho_id = $1 AND estado <> 'cancelada'`, trabalhoID).
		Scan(&a.ID, &a.Versao, &a.Estado, &a.PosicaoAtual, &a.ErroGeracao, &a.GeradaEm)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, `SELECT aa.ordem, aa.rotulo, u.nome_completo, aa.estado,
		to_char(aa.enviado_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI'), aa.signatario_id::text = $2
		FROM assinaturas_ata aa JOIN usuarios u ON u.id = aa.signatario_id WHERE aa.ata_id = $1 ORDER BY aa.ordem`, a.ID, usuarioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	minhaOrdem := 0
	for rows.Next() {
		var sr signatarioResumo
		if err := rows.Scan(&sr.Ordem, &sr.Rotulo, &sr.Nome, &sr.Estado, &sr.EnviadoEm, &sr.SouEu); err != nil {
			return nil, err
		}
		if sr.SouEu {
			minhaOrdem = sr.Ordem
		}
		a.Signatarios = append(a.Signatarios, sr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	a.SouSignatario = minhaOrdem > 0
	a.MinhaVez = a.Estado == "aguardando_assinaturas" && minhaOrdem == a.PosicaoAtual
	a.PodeBaixar = a.MinhaVez || (a.Estado == "finalizada" && a.SouSignatario)
	for _, sr := range a.Signatarios {
		if sr.Estado == "assinada" {
			a.AssinaturasColetadas++
		}
	}
	docenteSignatario := minhaOrdem >= 2
	a.PodeBaixarOriginal = docenteSignatario && a.Estado != "gerando"
	a.PodeBaixarAssinada = docenteSignatario && a.AssinaturasColetadas > 0
	switch {
	case a.Estado == "gerando" && a.ErroGeracao != "":
		a.Situacao = "Falha na geração da ATA"
	case a.Estado == "gerando":
		a.Situacao = "ATA sendo preparada"
	case a.Estado == "finalizada":
		a.Situacao = "ATA finalizada"
	case a.MinhaVez:
		a.Situacao = "Aguardando sua assinatura"
	case a.PosicaoAtual == 1:
		a.Situacao = "Aguardando assinatura do aluno"
	default:
		a.Situacao = "Aguardando assinaturas dos docentes"
	}
	return &a, nil
}

// trabalhoDoAluno devolve o trabalho mais recente não cancelado do aluno ("" se não houver).
func trabalhoDoAluno(ctx context.Context, q banco.Executor, alunoID string) (string, error) {
	var id string
	err := q.QueryRow(ctx, `SELECT id::text FROM trabalhos WHERE aluno_id = $1 AND estado <> 'cancelado' ORDER BY criado_em DESC LIMIT 1`, alunoID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// garantirTrabalho devolve o trabalho do aluno, criando-o (rascunho) se necessário.
func garantirTrabalho(ctx context.Context, tx pgx.Tx, alunoID, atorID string) (string, error) {
	if _, err := tx.Exec(ctx, `SELECT id FROM usuarios WHERE id = $1 FOR UPDATE`, alunoID); err != nil {
		return "", err
	}
	id, err := trabalhoDoAluno(ctx, tx, alunoID)
	if err != nil || id != "" {
		return id, err
	}
	var orientador *string
	err = tx.QueryRow(ctx, `SELECT docente_id::text FROM vinculos_orientacao WHERE aluno_id = $1 AND situacao = 'ativo'`, alunoID).Scan(&orientador)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if err := tx.QueryRow(ctx, `INSERT INTO trabalhos (aluno_id, orientador_id) VALUES ($1, $2) RETURNING id::text`, alunoID, orientador).Scan(&id); err != nil {
		return "", err
	}
	if orientador != nil {
		if _, err := tx.Exec(ctx, `INSERT INTO membros_banca (trabalho_id, docente_id, papel, ordem, obrigatorio, designado_por)
			VALUES ($1, $2, 'orientador', 0, true, $3)`, id, *orientador, atorID); err != nil {
			return "", err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO historico_estados_trabalho (trabalho_id, estado_anterior, estado_novo, ator_id, observacao)
		VALUES ($1, '', 'rascunho', $2, 'Trabalho criado')`, id, atorID); err != nil {
		return "", err
	}
	return id, nil
}

type historicoEstado struct {
	EstadoAnterior string `json:"estadoAnterior"`
	EstadoNovo     string `json:"estadoNovo"`
	Ator           *string `json:"ator"`
	Observacao     string `json:"observacao"`
	Data           string `json:"data"`
}

func listarHistorico(ctx context.Context, q banco.Executor, trabalhoID string) ([]historicoEstado, error) {
	rows, err := q.Query(ctx, `SELECT h.estado_anterior, h.estado_novo, u.nome_completo, h.observacao,
		to_char(h.criado_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI')
		FROM historico_estados_trabalho h LEFT JOIN usuarios u ON u.id = h.ator_id
		WHERE h.trabalho_id = $1 ORDER BY h.criado_em DESC, h.id DESC LIMIT 50`, trabalhoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	itens := []historicoEstado{}
	for rows.Next() {
		var h historicoEstado
		if err := rows.Scan(&h.EstadoAnterior, &h.EstadoNovo, &h.Ator, &h.Observacao, &h.Data); err != nil {
			return nil, err
		}
		itens = append(itens, h)
	}
	return itens, rows.Err()
}

type declaracaoResumo struct {
	GeradaID      *string `json:"geradaId"`
	GeradaEm      *string `json:"geradaEm"`
	AssinadaID    *string `json:"assinadaId"`
	AssinadaEm    *string `json:"assinadaEm"`
	AssinadaNome  *string `json:"assinadaNomeOriginal"`
	TarefaAtivaID *string `json:"tarefaAtivaId"`
}

func resumoDeclaracao(ctx context.Context, q banco.Executor, alunoID string, incluirGerada bool) (declaracaoResumo, error) {
	var d declaracaoResumo
	err := q.QueryRow(ctx, `SELECT
		(SELECT id::text FROM documentos_gerados WHERE tipo = 'declaracao_antiplagio' AND entidade_id = $1 ORDER BY versao DESC LIMIT 1),
		(SELECT to_char(gerado_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI') FROM documentos_gerados
		  WHERE tipo = 'declaracao_antiplagio' AND entidade_id = $1 ORDER BY versao DESC LIMIT 1),
		(SELECT id::text FROM declaracoes_assinadas WHERE aluno_id = $1 ORDER BY versao DESC LIMIT 1),
		(SELECT to_char(enviado_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI') FROM declaracoes_assinadas
		  WHERE aluno_id = $1 ORDER BY versao DESC LIMIT 1),
		(SELECT nome_original FROM declaracoes_assinadas WHERE aluno_id = $1 ORDER BY versao DESC LIMIT 1),
		(SELECT id::text FROM tarefas_documentos WHERE tipo = 'gerar_declaracao' AND entidade_id = $1 AND estado IN ('pendente', 'processando'))`,
		alunoID).Scan(&d.GeradaID, &d.GeradaEm, &d.AssinadaID, &d.AssinadaEm, &d.AssinadaNome, &d.TarefaAtivaID)
	if !incluirGerada {
		d.GeradaID, d.GeradaEm, d.TarefaAtivaID = nil, nil, nil
	}
	return d, err
}
