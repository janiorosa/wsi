package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"wsi/internal/documentos"
)

func itoa(i int) string { return strconv.Itoa(i) }

func (s *Servidor) adminPainel(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	proximos, err := listarEventos(ctx, s.pool, `WHERE e.data_apresentacao >= (now() AT TIME ZONE 'America/Sao_Paulo')::date - 1
		AND e.estado NOT IN ('encerrado', 'publicado') ORDER BY e.data_apresentacao LIMIT 6`)
	if err != nil {
		return err
	}
	var docentesPendentes, alunos, docentes, admins, tarefasAtivas, tarefasFalhas, criteriosAtivos, trabalhosSemEvento int
	if err := s.pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM perfis_docente WHERE situacao_aprovacao = 'pendente'),
		(SELECT count(*) FROM perfis_usuario WHERE perfil = 'ALUNO'),
		(SELECT count(*) FROM perfis_usuario WHERE perfil = 'DOCENTE'),
		(SELECT count(*) FROM perfis_usuario WHERE perfil = 'ADMIN'),
		(SELECT count(*) FROM tarefas_documentos WHERE estado IN ('pendente', 'processando')),
		(SELECT count(*) FROM tarefas_documentos WHERE estado = 'falhou' AND criado_em > now() - interval '30 days'),
		(SELECT count(*) FROM criterios_avaliacao WHERE ativo),
		(SELECT count(*) FROM trabalhos WHERE evento_id IS NULL AND estado NOT IN ('cancelado', 'concluido'))`).
		Scan(&docentesPendentes, &alunos, &docentes, &admins, &tarefasAtivas, &tarefasFalhas, &criteriosAtivos, &trabalhosSemEvento); err != nil {
		return err
	}
	return responderOK(w, map[string]any{
		"proximosEventos":    proximos,
		"docentesPendentes":  docentesPendentes,
		"usuarios":           map[string]int{"alunos": alunos, "docentes": docentes, "administradores": admins},
		"tarefasAtivas":      tarefasAtivas,
		"tarefasFalhas":      tarefasFalhas,
		"criteriosAtivos":    criteriosAtivos,
		"trabalhosSemEvento": trabalhosSemEvento,
	})
}

// ---------------------------------------------------------------------
// Critérios gerais (0 a 100, sem pesos)
// ---------------------------------------------------------------------

type criterioGeral struct {
	ID        string `json:"id"`
	Nome      string `json:"nome"`
	Descricao string `json:"descricao"`
	Ativo     bool   `json:"ativo"`
	Ordem     int    `json:"ordem"`
	Fixo      bool   `json:"fixo"`
}

func (s *Servidor) adminListarCriterios(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.pool.Query(r.Context(), `SELECT id::text, nome, descricao, ativo, ordem, fixo FROM criterios_avaliacao ORDER BY ordem, nome`)
	if err != nil {
		return err
	}
	defer rows.Close()
	itens := []criterioGeral{}
	for rows.Next() {
		var c criterioGeral
		if err := rows.Scan(&c.ID, &c.Nome, &c.Descricao, &c.Ativo, &c.Ordem, &c.Fixo); err != nil {
			return err
		}
		itens = append(itens, c)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return responderOK(w, itens)
}

func lerCriterio(r *http.Request) (criterioGeral, error) {
	var c criterioGeral
	if err := lerJSON(r, &c); err != nil {
		return c, err
	}
	c.Nome, c.Descricao = strings.TrimSpace(c.Nome), strings.TrimSpace(c.Descricao)
	if c.Nome == "" {
		return c, erroCampo("nome", "Informe o nome do critério.")
	}
	return c, nil
}

func (s *Servidor) adminCriarCriterio(w http.ResponseWriter, r *http.Request) error {
	c, err := lerCriterio(r)
	if err != nil {
		return err
	}
	var id string
	if err := s.pool.QueryRow(r.Context(), `INSERT INTO criterios_avaliacao (nome, descricao, ativo, ordem) VALUES ($1, $2, $3, $4) RETURNING id::text`,
		c.Nome, c.Descricao, c.Ativo, c.Ordem).Scan(&id); err != nil {
		return err
	}
	s.auditar(r.Context(), s.pool, r, "criar_criterio", "criterio", id, map[string]any{"nome": c.Nome, "ativo": c.Ativo})
	responderJSON(w, http.StatusCreated, map[string]string{"id": id})
	return nil
}

// Alterações não afetam trabalhos com avaliação finalizada (critérios congelados por trabalho).
func (s *Servidor) adminAtualizarCriterio(w http.ResponseWriter, r *http.Request) error {
	c, err := lerCriterio(r)
	if err != nil {
		return err
	}
	id := chi.URLParam(r, "id")
	var fixo bool
	var nomeAtual string
	if err := s.pool.QueryRow(r.Context(), `SELECT fixo, nome FROM criterios_avaliacao WHERE id = $1`, id).Scan(&fixo, &nomeAtual); err != nil {
		return erroNaoEncontrado("")
	}
	if fixo && (!c.Ativo || c.Nome != nomeAtual) {
		return erroConflito("O critério “Nota” é obrigatório: não pode ser renomeado nem desativado.")
	}
	tag, err := s.pool.Exec(r.Context(), `UPDATE criterios_avaliacao SET nome = $2, descricao = $3, ativo = $4, ordem = $5 WHERE id = $1`,
		id, c.Nome, c.Descricao, c.Ativo, c.Ordem)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return erroNaoEncontrado("")
	}
	s.auditar(r.Context(), s.pool, r, "atualizar_criterio", "criterio", id, map[string]any{"nome": c.Nome, "ativo": c.Ativo})
	return responderOK(w, map[string]bool{"ok": true})
}

// ---------------------------------------------------------------------
// Configurações institucionais (cabeçalho dos documentos e cidade)
// ---------------------------------------------------------------------

func (s *Servidor) adminObterConfiguracoes(w http.ResponseWriter, r *http.Request) error {
	cab, err := documentos.CarregarCabecalho(r.Context(), s.pool)
	if err != nil {
		return err
	}
	cidade, err := documentos.CarregarCidade(r.Context(), s.pool)
	if err != nil {
		return err
	}
	linhas := make([]documentos.LinhaCabecalhoJSON, 0, len(cab))
	for _, l := range cab {
		linhas = append(linhas, documentos.LinhaCabecalhoJSON{Texto: l.Texto, Negrito: l.Negrito})
	}
	return responderOK(w, map[string]any{"cabecalho": linhas, "cidade": cidade})
}

func (s *Servidor) adminSalvarConfiguracoes(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Cabecalho []documentos.LinhaCabecalhoJSON `json:"cabecalho"`
		Cidade    string                          `json:"cidade"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	var linhas []documentos.LinhaCabecalhoJSON
	for _, l := range req.Cabecalho {
		if t := strings.TrimSpace(l.Texto); t != "" {
			linhas = append(linhas, documentos.LinhaCabecalhoJSON{Texto: t, Negrito: l.Negrito})
		}
	}
	if len(linhas) == 0 || len(linhas) > 8 {
		return erroCampo("cabecalho", "O cabeçalho deve ter de 1 a 8 linhas.")
	}
	if strings.TrimSpace(req.Cidade) == "" {
		return erroCampo("cidade", "Informe a cidade.")
	}
	b, _ := json.Marshal(linhas)
	ses := sessaoDe(r)
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	for chave, valor := range map[string]string{documentos.ChaveCabecalho: string(b), documentos.ChaveCidade: strings.TrimSpace(req.Cidade)} {
		if _, err := tx.Exec(r.Context(), `INSERT INTO configuracoes_institucionais (chave, valor, atualizado_por) VALUES ($1, $2, $3)
			ON CONFLICT (chave) DO UPDATE SET valor = $2, atualizado_por = $3, atualizado_em = now()`, chave, valor, ses.UsuarioID); err != nil {
			return err
		}
	}
	if err := s.auditar(r.Context(), tx, r, "salvar_configuracoes", "configuracoes", "", map[string]any{"cabecalho": linhas, "cidade": req.Cidade}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}

// ---------------------------------------------------------------------
// Tarefas da fila e auditoria
// ---------------------------------------------------------------------

func (s *Servidor) adminListarTarefas(w http.ResponseWriter, r *http.Request) error {
	p := lerPaginacao(r)
	estado := r.URL.Query().Get("estado")
	var total int
	if err := s.pool.QueryRow(r.Context(), `SELECT count(*) FROM tarefas_documentos WHERE ($1 = '' OR estado = $1)`, estado).Scan(&total); err != nil {
		return err
	}
	rows, err := s.pool.Query(r.Context(), `SELECT t.id::text, t.tipo, t.estado, t.progresso, t.tentativas, t.erro, t.avisos, u.nome_completo,
		to_char(t.criado_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI:SS'),
		to_char(t.iniciado_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI:SS'),
		to_char(t.concluido_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI:SS')
		FROM tarefas_documentos t JOIN usuarios u ON u.id = t.solicitado_por
		WHERE ($1 = '' OR t.estado = $1) ORDER BY t.criado_em DESC LIMIT $2 OFFSET $3`, estado, p.Tamanho, p.Offset())
	if err != nil {
		return err
	}
	defer rows.Close()
	type tarefa struct {
		ID          string  `json:"id"`
		Tipo        string  `json:"tipo"`
		Estado      string  `json:"estado"`
		Progresso   int     `json:"progresso"`
		Tentativas  int     `json:"tentativas"`
		Erro        string  `json:"erro"`
		Avisos      string  `json:"avisos"`
		Solicitante string  `json:"solicitante"`
		CriadoEm    string  `json:"criadoEm"`
		IniciadoEm  *string `json:"iniciadoEm"`
		ConcluidoEm *string `json:"concluidoEm"`
	}
	var itens []tarefa
	for rows.Next() {
		var t tarefa
		if err := rows.Scan(&t.ID, &t.Tipo, &t.Estado, &t.Progresso, &t.Tentativas, &t.Erro, &t.Avisos, &t.Solicitante, &t.CriadoEm,
			&t.IniciadoEm, &t.ConcluidoEm); err != nil {
			return err
		}
		itens = append(itens, t)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return responderOK(w, novaPagina(itens, total, p))
}

// POST /admin/tarefas/{id}/reprocessar — recoloca uma tarefa que falhou na fila.
func (s *Servidor) adminReprocessarTarefa(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var tipo, entidade string
	var parametros map[string]string
	var parametrosTexto string
	if err := tx.QueryRow(ctx, `SELECT tipo, entidade_id::text, parametros::text FROM tarefas_documentos WHERE id = $1 AND estado = 'falhou' FOR UPDATE`, id).
		Scan(&tipo, &entidade, &parametrosTexto); err != nil {
		return erroNaoEncontrado("Tarefa com falha não encontrada.")
	}
	json.Unmarshal([]byte(parametrosTexto), &parametros)
	// Reprocessar somente se o objeto ainda aguarda o documento.
	switch tipo {
	case "gerar_ata":
		var estado string
		if err := tx.QueryRow(ctx, `SELECT estado FROM atas WHERE id = $1`, entidade).Scan(&estado); err != nil || estado != "gerando" {
			return erroConflito("Esta ATA não está mais aguardando geração.")
		}
		if _, err := tx.Exec(ctx, `UPDATE atas SET erro_geracao = '' WHERE id = $1`, entidade); err != nil {
			return err
		}
	case "gerar_anais", "gerar_apresentacoes":
		tag, err := tx.Exec(ctx, `UPDATE publicacoes_evento SET estado = 'em_preparacao', erro = '' WHERE id = $1 AND estado IN ('falhou', 'em_preparacao')`,
			parametros["publicacao_id"])
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return erroConflito("Esta publicação não pode mais ser reprocessada.")
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE tarefas_documentos SET estado = 'pendente', erro = '', progresso = 0, concluido_em = NULL WHERE id = $1`, id); err != nil {
		return err
	}
	if err := s.auditar(ctx, tx, r, "reprocessar_tarefa", "tarefa", id, map[string]any{"tipo": tipo}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.fila.Notificar()
	return responderOK(w, map[string]bool{"ok": true})
}

func (s *Servidor) adminListarAuditoria(w http.ResponseWriter, r *http.Request) error {
	p := lerPaginacao(r)
	busca := "%" + strings.ToLower(strings.TrimSpace(r.URL.Query().Get("busca"))) + "%"
	var total int
	filtro := ` WHERE (lower(ra.acao) LIKE $1 OR lower(ra.entidade) LIKE $1 OR ra.entidade_id LIKE $1 OR lower(COALESCE(u.nome_completo, '')) LIKE $1)`
	if err := s.pool.QueryRow(r.Context(), `SELECT count(*) FROM registros_auditoria ra LEFT JOIN usuarios u ON u.id = ra.ator_id`+filtro, busca).Scan(&total); err != nil {
		return err
	}
	rows, err := s.pool.Query(r.Context(), `SELECT ra.id, COALESCE(u.nome_completo, 'sistema'), ra.acao, ra.entidade, ra.entidade_id, ra.dados::text, ra.ip,
		to_char(ra.criado_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI:SS')
		FROM registros_auditoria ra LEFT JOIN usuarios u ON u.id = ra.ator_id`+filtro+` ORDER BY ra.id DESC LIMIT $2 OFFSET $3`, busca, p.Tamanho, p.Offset())
	if err != nil {
		return err
	}
	defer rows.Close()
	type registro struct {
		ID         int64           `json:"id"`
		Ator       string          `json:"ator"`
		Acao       string          `json:"acao"`
		Entidade   string          `json:"entidade"`
		EntidadeID string          `json:"entidadeId"`
		Dados      json.RawMessage `json:"dados"`
		IP         string          `json:"ip"`
		Data       string          `json:"data"`
	}
	var itens []registro
	for rows.Next() {
		var reg registro
		var dados string
		if err := rows.Scan(&reg.ID, &reg.Ator, &reg.Acao, &reg.Entidade, &reg.EntidadeID, &dados, &reg.IP, &reg.Data); err != nil {
			return err
		}
		reg.Dados = json.RawMessage(dados)
		itens = append(itens, reg)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return responderOK(w, novaPagina(itens, total, p))
}

// GET /admin/trabalhos?busca= — acompanhamento de trabalhos (sem notas, avaliações ou ATAs).
func (s *Servidor) adminListarTrabalhos(w http.ResponseWriter, r *http.Request) error {
	busca := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("busca")))
	rows, err := s.pool.Query(r.Context(), `SELECT t.id::text, t.titulo, a.nome_completo, o.nome_completo, e.nome, t.estado, t.excecao_evento_passado
		FROM trabalhos t JOIN usuarios a ON a.id = t.aluno_id LEFT JOIN usuarios o ON o.id = t.orientador_id LEFT JOIN eventos e ON e.id = t.evento_id
		WHERE t.estado <> 'cancelado' AND ($1 = '' OR lower(t.titulo) LIKE '%' || $1 || '%' OR lower(a.nome_completo) LIKE '%' || $1 || '%')
		ORDER BY a.nome_completo LIMIT 50`, busca)
	if err != nil {
		return err
	}
	defer rows.Close()
	type trabalho struct {
		ID         string  `json:"id"`
		Titulo     string  `json:"titulo"`
		Aluno      string  `json:"aluno"`
		Orientador *string `json:"orientador"`
		Evento     *string `json:"evento"`
		Estado     string  `json:"estado"`
		Excecao    string  `json:"excecaoEventoPassado"`
	}
	itens := []trabalho{}
	for rows.Next() {
		var t trabalho
		if err := rows.Scan(&t.ID, &t.Titulo, &t.Aluno, &t.Orientador, &t.Evento, &t.Estado, &t.Excecao); err != nil {
			return err
		}
		itens = append(itens, t)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return responderOK(w, itens)
}
