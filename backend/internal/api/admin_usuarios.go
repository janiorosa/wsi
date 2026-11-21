package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type itemUsuario struct {
	ID              string   `json:"id"`
	Nome            string   `json:"nome"`
	Email           string   `json:"email"`
	Telefone        string   `json:"telefone"`
	Identificador   string   `json:"identificador"`
	TipoIdentificador string `json:"tipoIdentificador"`
	Perfis          []string `json:"perfis"`
	Situacao        string   `json:"situacao"`
	SituacaoDocente string   `json:"situacaoDocente"`
	DadosPendentes  bool     `json:"dadosPendentesConferencia"`
	Orientador      *string  `json:"orientador"`
	CriadoEm        string   `json:"criadoEm"`
}

const selecaoUsuario = `SELECT u.id::text, u.nome_completo, u.email, u.telefone, u.identificador_institucional, u.tipo_identificador,
	COALESCE((SELECT array_agg(perfil ORDER BY perfil) FROM perfis_usuario WHERE usuario_id = u.id), '{}'::text[]),
	u.situacao, COALESCE(pd.situacao_aprovacao, ''), u.dados_pendentes_conferencia,
	(SELECT d.nome_completo FROM vinculos_orientacao v JOIN usuarios d ON d.id = v.docente_id WHERE v.aluno_id = u.id AND v.situacao = 'ativo'),
	to_char(u.criado_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY')
	FROM usuarios u LEFT JOIN perfis_docente pd ON pd.usuario_id = u.id`

func escanearUsuario(row interface{ Scan(...any) error }) (itemUsuario, error) {
	var u itemUsuario
	err := row.Scan(&u.ID, &u.Nome, &u.Email, &u.Telefone, &u.Identificador, &u.TipoIdentificador, &u.Perfis, &u.Situacao,
		&u.SituacaoDocente, &u.DadosPendentes, &u.Orientador, &u.CriadoEm)
	return u, err
}

// GET /admin/usuarios?busca=&perfil=ADMIN|DOCENTE|ALUNO|PENDENTE&pagina=&tamanho=&ordem=nome|-criado
func (s *Servidor) adminListarUsuarios(w http.ResponseWriter, r *http.Request) error {
	p := lerPaginacao(r)
	busca := "%" + strings.ToLower(strings.TrimSpace(r.URL.Query().Get("busca"))) + "%"
	perfil := r.URL.Query().Get("perfil")
	filtro := ` WHERE (lower(u.nome_completo) LIKE $1 OR u.email LIKE $1 OR lower(u.identificador_institucional) LIKE $1)
		AND ($2 = '' OR ($2 = 'PENDENTE' AND pd.situacao_aprovacao = 'pendente')
		     OR EXISTS (SELECT 1 FROM perfis_usuario WHERE usuario_id = u.id AND perfil = $2))`
	var total int
	if err := s.pool.QueryRow(r.Context(), `SELECT count(*) FROM usuarios u LEFT JOIN perfis_docente pd ON pd.usuario_id = u.id`+filtro,
		busca, perfil).Scan(&total); err != nil {
		return err
	}
	ordem := ordenacao(r, map[string]string{"nome": "u.nome_completo", "email": "u.email", "criado": "u.criado_em"}, "u.nome_completo ASC")
	rows, err := s.pool.Query(r.Context(), selecaoUsuario+filtro+` ORDER BY `+ordem+` LIMIT $3 OFFSET $4`, busca, perfil, p.Tamanho, p.Offset())
	if err != nil {
		return err
	}
	defer rows.Close()
	var itens []itemUsuario
	for rows.Next() {
		u, err := escanearUsuario(rows)
		if err != nil {
			return err
		}
		itens = append(itens, u)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return responderOK(w, novaPagina(itens, total, p))
}

func (s *Servidor) adminObterUsuario(w http.ResponseWriter, r *http.Request) error {
	u, err := escanearUsuario(s.pool.QueryRow(r.Context(), selecaoUsuario+` WHERE u.id = $1`, chi.URLParam(r, "id")))
	if err != nil {
		return err
	}
	return responderOK(w, u)
}

// POST /admin/usuarios — cadastro direto pelo ADMIN (aluno, docente já aprovado, ou administrador).
func (s *Servidor) adminCriarUsuario(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		dadosCadastro
		Tipo        string `json:"tipo"` // ALUNO | DOCENTE | ADMIN
		TambemAdmin bool   `json:"tambemAdmin"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	d := req.dadosCadastro
	d.normalizar()
	if req.Tipo != PerfilAluno && req.Tipo != PerfilDocente && req.Tipo != PerfilAdmin {
		return erroCampo("tipo", "Escolha o tipo de usuário.")
	}
	// Matrícula/SIAPE é obrigatório: é com ele que a pessoa faz o primeiro acesso.
	if err := d.validar(false, "a matrícula ou o SIAPE", true); err != nil {
		return err
	}
	ses := sessaoDe(r)
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var id string
	switch req.Tipo {
	case PerfilAluno:
		id, err = inserirAluno(r.Context(), tx, d, "", ses.UsuarioID)
	case PerfilDocente:
		id, err = inserirUsuario(r.Context(), tx, d, "SIAPE", "")
		if err == nil {
			_, err = tx.Exec(r.Context(), `INSERT INTO perfis_docente (usuario_id, siape, situacao_aprovacao, aprovado_por, aprovado_em)
				VALUES ($1, $2, 'aprovado', $3, now())`, id, d.Identificador, ses.UsuarioID)
		}
		if err == nil {
			_, err = tx.Exec(r.Context(), `INSERT INTO perfis_usuario (usuario_id, perfil, concedido_por) VALUES ($1, 'DOCENTE', $2)`, id, ses.UsuarioID)
		}
		if err == nil && req.TambemAdmin {
			_, err = tx.Exec(r.Context(), `INSERT INTO perfis_usuario (usuario_id, perfil, concedido_por) VALUES ($1, 'ADMIN', $2)`, id, ses.UsuarioID)
		}
	case PerfilAdmin:
		id, err = inserirUsuario(r.Context(), tx, d, "", "")
		if err == nil {
			_, err = tx.Exec(r.Context(), `INSERT INTO perfis_usuario (usuario_id, perfil, concedido_por) VALUES ($1, 'ADMIN', $2)`, id, ses.UsuarioID)
		}
	}
	if err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "criar_usuario", "usuario", id, map[string]any{"tipo": req.Tipo, "admin": req.TambemAdmin}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	responderJSON(w, http.StatusCreated, map[string]any{"id": id})
	return nil
}

func (s *Servidor) adminAtualizarUsuario(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	var req struct {
		Nome          string `json:"nome"`
		Email         string `json:"email"`
		Telefone      string `json:"telefone"`
		Identificador string `json:"identificador"`
		Situacao      string `json:"situacao"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	req.Nome = strings.Join(strings.Fields(req.Nome), " ")
	req.Email = normalizarEmail(req.Email)
	if len(req.Nome) < 3 {
		return erroCampo("nome", "Informe o nome completo.")
	}
	if !validarEmail(req.Email) {
		return erroCampo("email", "Informe um e-mail válido.")
	}
	if req.Situacao != "ativo" && req.Situacao != "bloqueado" && req.Situacao != "aguardando_primeiro_acesso" {
		return erroCampo("situacao", "Situação inválida.")
	}
	ses := sessaoDe(r)
	if id == ses.UsuarioID && req.Situacao == "bloqueado" {
		return erroRequisicao("Você não pode bloquear a própria conta.")
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var anterior map[string]any
	var nomeAnt, emailAnt, situacaoAnt string
	if err := tx.QueryRow(r.Context(), `SELECT nome_completo, email, situacao FROM usuarios WHERE id = $1 FOR UPDATE`, id).
		Scan(&nomeAnt, &emailAnt, &situacaoAnt); err != nil {
		return err
	}
	anterior = map[string]any{"nome": nomeAnt, "email": emailAnt, "situacao": situacaoAnt}
	// "aguardando_primeiro_acesso" só é mantido se o usuário ainda não tem senha.
	if _, err := tx.Exec(r.Context(), `UPDATE usuarios SET nome_completo = $2, email = $3, telefone = $4, identificador_institucional = $5,
		situacao = CASE WHEN $6 = 'aguardando_primeiro_acesso' AND hash_senha IS NOT NULL THEN 'ativo' ELSE $6 END,
		dados_pendentes_conferencia = false, atualizado_em = now() WHERE id = $1`,
		id, req.Nome, req.Email, strings.TrimSpace(req.Telefone), strings.TrimSpace(req.Identificador), req.Situacao); err != nil {
		return err
	}
	if _, err := tx.Exec(r.Context(), `UPDATE perfis_aluno SET matricula = $2 WHERE usuario_id = $1`, id, strings.TrimSpace(req.Identificador)); err != nil {
		return err
	}
	if _, err := tx.Exec(r.Context(), `UPDATE perfis_docente SET siape = $2 WHERE usuario_id = $1`, id, strings.TrimSpace(req.Identificador)); err != nil {
		return err
	}
	if req.Situacao == "bloqueado" {
		if _, err := tx.Exec(r.Context(), `DELETE FROM sessoes WHERE usuario_id = $1`, id); err != nil {
			return err
		}
	}
	if err := s.auditar(r.Context(), tx, r, "atualizar_usuario", "usuario", id, map[string]any{"antes": anterior,
		"depois": map[string]any{"nome": req.Nome, "email": req.Email, "situacao": req.Situacao}}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}

// POST /admin/usuarios/{id}/perfis {perfil: ADMIN|DOCENTE}
func (s *Servidor) adminAdicionarPerfil(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	var req struct {
		Perfil string `json:"perfil"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	if req.Perfil != PerfilAdmin && req.Perfil != PerfilDocente {
		return erroCampo("perfil", "Somente os perfis ADMIN e DOCENTE podem ser atribuídos.")
	}
	ses := sessaoDe(r)
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if aluno, err := possuiPerfil(r.Context(), tx, id, PerfilAluno); err != nil {
		return err
	} else if aluno {
		return erroConflito("Contas de ALUNO não podem receber outros perfis.")
	}
	if req.Perfil == PerfilDocente {
		if _, err := tx.Exec(r.Context(), `INSERT INTO perfis_docente (usuario_id, situacao_aprovacao, aprovado_por, aprovado_em)
			VALUES ($1, 'aprovado', $2, now())
			ON CONFLICT (usuario_id) DO UPDATE SET situacao_aprovacao = 'aprovado', aprovado_por = $2, aprovado_em = now(), motivo_rejeicao = ''`,
			id, ses.UsuarioID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(r.Context(), `INSERT INTO perfis_usuario (usuario_id, perfil, concedido_por) VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, id, req.Perfil, ses.UsuarioID); err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "atribuir_perfil", "usuario", id, map[string]any{"perfil": req.Perfil}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}

func (s *Servidor) adminRemoverPerfil(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	perfil := chi.URLParam(r, "perfil")
	if perfil != PerfilAdmin && perfil != PerfilDocente {
		return erroRequisicao("Somente os perfis ADMIN e DOCENTE podem ser removidos.")
	}
	ses := sessaoDe(r)
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if perfil == PerfilAdmin {
		var admins int
		if err := tx.QueryRow(r.Context(), `SELECT count(*) FROM perfis_usuario WHERE perfil = 'ADMIN'`).Scan(&admins); err != nil {
			return err
		}
		if admins <= 1 {
			return erroConflito("Não é possível remover o último administrador do sistema.")
		}
	}
	tag, err := tx.Exec(r.Context(), `DELETE FROM perfis_usuario WHERE usuario_id = $1 AND perfil = $2`, id, perfil)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return erroNaoEncontrado("O usuário não possui este perfil.")
	}
	// Sessões abertas naquele contexto deixam de valer imediatamente.
	if _, err := tx.Exec(r.Context(), `UPDATE sessoes SET contexto = NULL WHERE usuario_id = $1 AND contexto = $2`, id, perfil); err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "remover_perfil", "usuario", id, map[string]any{"perfil": perfil, "proprio": id == ses.UsuarioID}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}

// POST /admin/usuarios/{id}/redefinir-acesso — apaga a senha e encerra as sessões.
// A pessoa volta a fazer o "Primeiro acesso" (nome completo, e-mail e matrícula/SIAPE).
func (s *Servidor) adminRedefinirAcesso(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	ses := sessaoDe(r)
	if id == ses.UsuarioID {
		return erroRequisicao("Você não pode redefinir o acesso da sua própria conta. Peça a outro administrador ou use “Minha conta” para trocar a senha.")
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	tag, err := tx.Exec(r.Context(), `UPDATE usuarios SET hash_senha = NULL,
		situacao = CASE WHEN situacao = 'bloqueado' THEN situacao ELSE 'aguardando_primeiro_acesso' END,
		tentativas_falhas = 0, bloqueado_ate = NULL, atualizado_em = now() WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return erroNaoEncontrado("Usuário não encontrado.")
	}
	if _, err := tx.Exec(r.Context(), `DELETE FROM sessoes WHERE usuario_id = $1`, id); err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "redefinir_acesso", "usuario", id, nil); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}

func (s *Servidor) adminAprovarDocente(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	ses := sessaoDe(r)
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	tag, err := tx.Exec(r.Context(), `UPDATE perfis_docente SET situacao_aprovacao = 'aprovado', aprovado_por = $2, aprovado_em = now(), motivo_rejeicao = ''
		WHERE usuario_id = $1`, id, ses.UsuarioID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return erroNaoEncontrado("Solicitação de docente não encontrada.")
	}
	if _, err := tx.Exec(r.Context(), `INSERT INTO perfis_usuario (usuario_id, perfil, concedido_por) VALUES ($1, 'DOCENTE', $2) ON CONFLICT DO NOTHING`,
		id, ses.UsuarioID); err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "aprovar_docente", "usuario", id, nil); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}

func (s *Servidor) adminRejeitarDocente(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	var req struct {
		Motivo string `json:"motivo"`
	}
	lerJSON(r, &req)
	ses := sessaoDe(r)
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	tag, err := tx.Exec(r.Context(), `UPDATE perfis_docente SET situacao_aprovacao = 'rejeitado', aprovado_por = $2, aprovado_em = now(), motivo_rejeicao = $3
		WHERE usuario_id = $1 AND situacao_aprovacao = 'pendente'`, id, ses.UsuarioID, strings.TrimSpace(req.Motivo))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return erroNaoEncontrado("Solicitação pendente não encontrada.")
	}
	if err := s.auditar(r.Context(), tx, r, "rejeitar_docente", "usuario", id, map[string]any{"motivo": req.Motivo}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}

// listarDocentesAprovados é usado pelo aluno (escolher orientador) e pelo docente (montar banca).
func (s *Servidor) listarDocentesAprovados(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.pool.Query(r.Context(), `SELECT u.id::text, u.nome_completo FROM usuarios u
		JOIN perfis_usuario pu ON pu.usuario_id = u.id AND pu.perfil = 'DOCENTE'
		WHERE u.situacao <> 'bloqueado' ORDER BY u.nome_completo`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type docente struct {
		ID   string `json:"id"`
		Nome string `json:"nome"`
	}
	itens := []docente{}
	for rows.Next() {
		var d docente
		if err := rows.Scan(&d.ID, &d.Nome); err != nil {
			return err
		}
		itens = append(itens, d)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return responderOK(w, itens)
}
