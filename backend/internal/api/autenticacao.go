package api

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"

	"wsi/internal/banco"
	"wsi/internal/seguranca"
)

type usuarioSessao struct {
	ID    string `json:"id"`
	Nome  string `json:"nome"`
	Email string `json:"email"`
}

type respostaSessao struct {
	Autenticado       bool           `json:"autenticado"`
	Usuario           *usuarioSessao `json:"usuario,omitempty"`
	Perfis            []string       `json:"perfis"`
	Contexto          string         `json:"contexto"`
	PrecisaEscolher   bool           `json:"precisaEscolherContexto"`
	SituacaoDocente   string         `json:"situacaoDocente"`
	EhApresentador    bool           `json:"ehApresentador"`
	CSRF              string         `json:"csrf,omitempty"`
}

// contextoAutomatico: somente ADMIN -> ADMIN; somente DOCENTE -> DOCENTE; ALUNO -> ALUNO;
// ADMIN + DOCENTE -> "" (o usuário escolhe após o login).
func contextoAutomatico(perfis []string) string {
	admin, docente := slices.Contains(perfis, PerfilAdmin), slices.Contains(perfis, PerfilDocente)
	switch {
	case slices.Contains(perfis, PerfilAluno):
		return PerfilAluno
	case admin && docente:
		return ""
	case admin:
		return PerfilAdmin
	case docente:
		return PerfilDocente
	}
	return ""
}

func (s *Servidor) montarRespostaSessao(ctx context.Context, ses *Sessao) (respostaSessao, error) {
	if ses == nil {
		return respostaSessao{Autenticado: false, Perfis: []string{}}, nil
	}
	resp := respostaSessao{
		Autenticado: true,
		Usuario:     &usuarioSessao{ID: ses.UsuarioID, Nome: ses.Nome, Email: ses.Email},
		Perfis:      ses.Perfis,
		Contexto:    ses.Contexto,
		CSRF:        ses.CSRF,
	}
	if resp.Perfis == nil {
		resp.Perfis = []string{}
	}
	resp.PrecisaEscolher = ses.Contexto == "" && ses.TemPerfil(PerfilAdmin) && ses.TemPerfil(PerfilDocente)
	err := s.pool.QueryRow(ctx, `SELECT
		COALESCE((SELECT situacao_aprovacao FROM perfis_docente WHERE usuario_id = $1), ''),
		EXISTS(SELECT 1 FROM apresentadores_evento WHERE docente_id = $1)`, ses.UsuarioID).Scan(&resp.SituacaoDocente, &resp.EhApresentador)
	return resp, err
}

func (s *Servidor) obterSessao(w http.ResponseWriter, r *http.Request) error {
	resp, err := s.montarRespostaSessao(r.Context(), sessaoDe(r))
	if err != nil {
		return err
	}
	return responderOK(w, resp)
}

// criarSessao grava a sessão (somente o hash do token) e define o cookie HttpOnly.
func (s *Servidor) criarSessao(ctx context.Context, w http.ResponseWriter, usuarioID string, perfis []string) (*Sessao, error) {
	token, hash, err := seguranca.GerarToken()
	if err != nil {
		return nil, err
	}
	csrf, _, err := seguranca.GerarToken()
	if err != nil {
		return nil, err
	}
	// Limpeza oportunista de sessões expiradas (volume pequeno; dispensa rotina agendada).
	if _, err := s.pool.Exec(ctx, `DELETE FROM sessoes WHERE expira_em < now()`); err != nil {
		return nil, err
	}
	contexto := contextoAutomatico(perfis)
	ses := &Sessao{UsuarioID: usuarioID, Perfis: perfis, Contexto: contexto, CSRF: csrf}
	err = s.pool.QueryRow(ctx, `INSERT INTO sessoes (hash_token, usuario_id, contexto, token_csrf, expira_em)
		VALUES ($1, $2, $3, $4, now() + interval '12 hours') RETURNING id::text`,
		hash, usuarioID, textoOuNil(contexto), csrf).Scan(&ses.ID)
	if err != nil {
		return nil, err
	}
	if err := s.pool.QueryRow(ctx, `SELECT nome_completo, email FROM usuarios WHERE id = $1`, usuarioID).Scan(&ses.Nome, &ses.Email); err != nil {
		return nil, err
	}
	definirCookie(w, token, s.cfg.CookieSeguro)
	return ses, nil
}

type requisicaoEntrar struct {
	Email string `json:"email"`
	Senha string `json:"senha"`
}

const (
	maximoTentativas = 5
	tempoBloqueio    = 15 * time.Minute
)

func (s *Servidor) entrar(w http.ResponseWriter, r *http.Request) error {
	var req requisicaoEntrar
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	email := normalizarEmail(req.Email)
	falha := &ErroAPI{Status: http.StatusUnauthorized, Codigo: "credenciais_invalidas", Mensagem: "E-mail ou senha incorretos."}

	var id, situacao string
	var hash *string
	var tentativas int
	var bloqueadoAte *time.Time
	err := s.pool.QueryRow(r.Context(), `SELECT id::text, situacao, hash_senha, tentativas_falhas, bloqueado_ate FROM usuarios WHERE email = $1`, email).
		Scan(&id, &situacao, &hash, &tentativas, &bloqueadoAte)
	if errors.Is(err, pgx.ErrNoRows) {
		// Custo equivalente ao de uma verificação real, para não revelar se o e-mail existe.
		seguranca.VerificarSenha(req.Senha, "$argon2id$v=19$m=65536,t=3,p=2$c2FsdGVzdGVzYWx0ZXN0$aGFzaGhhc2hoYXNoaGFzaGhhc2hoYXNoaGFzaGhhc2g")
		return falha
	}
	if err != nil {
		return err
	}
	if bloqueadoAte != nil && bloqueadoAte.After(time.Now()) {
		return &ErroAPI{Status: http.StatusTooManyRequests, Codigo: "conta_bloqueada_temporariamente",
			Mensagem: "Muitas tentativas incorretas. Tente novamente em alguns minutos."}
	}
	if situacao == "aguardando_primeiro_acesso" || hash == nil {
		return &ErroAPI{Status: http.StatusUnauthorized, Codigo: "primeiro_acesso_pendente",
			Mensagem: "Esta conta ainda não tem senha. Use a opção “Primeiro acesso” para criá-la."}
	}
	if situacao == "bloqueado" {
		return &ErroAPI{Status: http.StatusForbidden, Codigo: "conta_bloqueada", Mensagem: "Conta bloqueada. Procure a administração."}
	}
	if !seguranca.VerificarSenha(req.Senha, *hash) {
		tentativas++
		var bloqueio any
		if tentativas >= maximoTentativas {
			bloqueio = time.Now().Add(tempoBloqueio)
			tentativas = 0
		}
		s.pool.Exec(r.Context(), `UPDATE usuarios SET tentativas_falhas = $2, bloqueado_ate = $3 WHERE id = $1`, id, tentativas, bloqueio)
		return falha
	}
	if _, err := s.pool.Exec(r.Context(), `UPDATE usuarios SET tentativas_falhas = 0, bloqueado_ate = NULL WHERE id = $1`, id); err != nil {
		return err
	}
	perfis, err := carregarPerfis(r.Context(), s.pool, id)
	if err != nil {
		return err
	}
	ses, err := s.criarSessao(r.Context(), w, id, perfis)
	if err != nil {
		return err
	}
	s.auditar(r.Context(), s.pool, r.WithContext(context.WithValue(r.Context(), chaveContexto{}, ses)), "login", "usuario", id, nil)
	resp, err := s.montarRespostaSessao(r.Context(), ses)
	if err != nil {
		return err
	}
	return responderOK(w, resp)
}

func carregarPerfis(ctx context.Context, q banco.Executor, usuarioID string) ([]string, error) {
	var perfis []string
	err := q.QueryRow(ctx, `SELECT COALESCE(array_agg(perfil ORDER BY perfil), '{}'::text[]) FROM perfis_usuario WHERE usuario_id = $1`, usuarioID).Scan(&perfis)
	return perfis, err
}

func (s *Servidor) sair(w http.ResponseWriter, r *http.Request) error {
	if ses := sessaoDe(r); ses != nil {
		if _, err := s.pool.Exec(r.Context(), `DELETE FROM sessoes WHERE id = $1`, ses.ID); err != nil {
			return err
		}
	}
	apagarCookie(w, s.cfg.CookieSeguro)
	return responderOK(w, map[string]bool{"ok": true})
}

func (s *Servidor) escolherContexto(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	var req struct {
		Contexto string `json:"contexto"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	if req.Contexto != PerfilAdmin && req.Contexto != PerfilDocente && req.Contexto != PerfilAluno {
		return erroCampo("contexto", "Contexto inválido.")
	}
	if !ses.TemPerfil(req.Contexto) {
		return erroProibido("Você não possui o perfil " + req.Contexto + ".")
	}
	// A escolha altera somente a sessão; os perfis armazenados não mudam.
	if _, err := s.pool.Exec(r.Context(), `UPDATE sessoes SET contexto = $2 WHERE id = $1`, ses.ID, req.Contexto); err != nil {
		return err
	}
	anterior := ses.Contexto
	ses.Contexto = req.Contexto
	s.auditar(r.Context(), s.pool, r, "trocar_contexto", "sessao", ses.ID, map[string]any{"anterior": anterior})
	resp, err := s.montarRespostaSessao(r.Context(), ses)
	if err != nil {
		return err
	}
	return responderOK(w, resp)
}

// ---------------------------------------------------------------------
// Cadastro
// ---------------------------------------------------------------------

func normalizarEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

// validarEmail confere apenas a sintaxe. Não há confirmação por e-mail nesta versão.
func validarEmail(email string) bool {
	if len(email) > 254 || strings.ContainsAny(email, " <>\"") {
		return false
	}
	end, err := mail.ParseAddress(email)
	if err != nil || end.Address != email {
		return false
	}
	partes := strings.Split(email, "@")
	return len(partes) == 2 && partes[0] != "" && strings.Contains(partes[1], ".") && !strings.HasSuffix(partes[1], ".")
}

type dadosCadastro struct {
	Nome          string `json:"nome"`
	Email         string `json:"email"`
	Telefone      string `json:"telefone"`
	Identificador string `json:"identificador"` // matrícula (aluno) ou SIAPE (docente)
	Curso         string `json:"curso"`
	Senha         string `json:"senha"`
}

func (d *dadosCadastro) normalizar() {
	d.Nome = strings.Join(strings.Fields(d.Nome), " ")
	d.Email = normalizarEmail(d.Email)
	d.Telefone = strings.TrimSpace(d.Telefone)
	d.Identificador = strings.TrimSpace(d.Identificador)
	d.Curso = strings.TrimSpace(d.Curso)
}

func (d dadosCadastro) validar(exigirSenha bool, rotuloIdentificador string, exigirIdentificador bool) error {
	campos := map[string]string{}
	if len(d.Nome) < 3 {
		campos["nome"] = "Informe o nome completo."
	}
	if !validarEmail(d.Email) {
		campos["email"] = "Informe um e-mail válido."
	}
	if exigirIdentificador && d.Identificador == "" {
		campos["identificador"] = "Informe " + rotuloIdentificador + "."
	}
	if len(d.Telefone) > 30 {
		campos["telefone"] = "Telefone muito longo."
	}
	if exigirSenha {
		if err := seguranca.ValidarForcaSenha(d.Senha); err != nil {
			campos["senha"] = "A senha deve ter pelo menos 8 caracteres."
		}
	}
	if len(campos) > 0 {
		return erroValidacao("Verifique os campos destacados.", campos)
	}
	return nil
}

// inserirUsuario cria o usuário; hash vazio = conta aguardando primeiro acesso.
func inserirUsuario(ctx context.Context, q banco.Executor, d dadosCadastro, tipoIdentificador, hash string) (string, error) {
	situacao := "ativo"
	var hashSenha any = hash
	if hash == "" {
		situacao = "aguardando_primeiro_acesso"
		hashSenha = nil
	}
	var id string
	err := q.QueryRow(ctx, `INSERT INTO usuarios (nome_completo, email, telefone, identificador_institucional, tipo_identificador, hash_senha, situacao)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id::text`,
		d.Nome, d.Email, d.Telefone, d.Identificador, tipoIdentificador, hashSenha, situacao).Scan(&id)
	return id, err
}

func inserirAluno(ctx context.Context, q banco.Executor, d dadosCadastro, hash string, concedidoPor any) (string, error) {
	id, err := inserirUsuario(ctx, q, d, "MATRICULA", hash)
	if err != nil {
		return "", err
	}
	curso := d.Curso
	if curso == "" {
		curso = "Sistemas de Informação"
	}
	if _, err := q.Exec(ctx, `INSERT INTO perfis_aluno (usuario_id, matricula, curso) VALUES ($1, $2, $3)`, id, d.Identificador, curso); err != nil {
		return "", err
	}
	_, err = q.Exec(ctx, `INSERT INTO perfis_usuario (usuario_id, perfil, concedido_por) VALUES ($1, 'ALUNO', $2)`, id, concedidoPor)
	return id, err
}

// Aluno: cadastro livre, conta ativa imediatamente, sem aprovação e sem confirmação de e-mail.
func (s *Servidor) cadastrarAluno(w http.ResponseWriter, r *http.Request) error {
	var d dadosCadastro
	if err := lerJSON(r, &d); err != nil {
		return err
	}
	d.normalizar()
	if err := d.validar(true, "a matrícula", true); err != nil {
		return err
	}
	hash, err := seguranca.GerarHashSenha(d.Senha)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	id, err := inserirAluno(r.Context(), tx, d, hash, nil)
	if err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	ses, err := s.criarSessao(r.Context(), w, id, []string{PerfilAluno})
	if err != nil {
		return err
	}
	resp, err := s.montarRespostaSessao(r.Context(), ses)
	if err != nil {
		return err
	}
	responderJSON(w, http.StatusCreated, resp)
	return nil
}

// Docente: cadastro livre, mas o perfil DOCENTE só é concedido após aprovação do ADMIN.
func (s *Servidor) cadastrarDocente(w http.ResponseWriter, r *http.Request) error {
	var d dadosCadastro
	if err := lerJSON(r, &d); err != nil {
		return err
	}
	d.normalizar()
	if err := d.validar(true, "o SIAPE", false); err != nil {
		return err
	}
	hash, err := seguranca.GerarHashSenha(d.Senha)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	id, err := inserirUsuario(r.Context(), tx, d, "SIAPE", hash)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(r.Context(), `INSERT INTO perfis_docente (usuario_id, siape, situacao_aprovacao) VALUES ($1, $2, 'pendente')`, id, d.Identificador); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	ses, err := s.criarSessao(r.Context(), w, id, []string{})
	if err != nil {
		return err
	}
	resp, err := s.montarRespostaSessao(r.Context(), ses)
	if err != nil {
		return err
	}
	responderJSON(w, http.StatusCreated, resp)
	return nil
}

// ---------------------------------------------------------------------
// Primeiro acesso: a própria pessoa cria a senha informando nome completo,
// e-mail e matrícula (aluno) ou SIAPE (docente). Só funciona enquanto a conta
// não tem senha. Depois disso, somente o ADMIN pode "redefinir o acesso",
// o que apaga a senha e permite um novo primeiro acesso.
// ---------------------------------------------------------------------

// normalizarNome: minúsculas, sem acentos e com espaços simples.
func normalizarNome(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// normalizarIdentificador: somente letras e números, sem zeros à esquerda ("0013.130" == "13130").
func normalizarIdentificador(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return strings.TrimLeft(b.String(), "0")
}

func (s *Servidor) primeiroAcesso(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Nome          string `json:"nome"`
		Email         string `json:"email"`
		Identificador string `json:"identificador"`
		Senha         string `json:"senha"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	if err := seguranca.ValidarForcaSenha(req.Senha); err != nil {
		return erroCampo("senha", "A senha deve ter pelo menos 8 caracteres.")
	}
	naoConfere := erroRequisicao("Os dados informados não conferem com nenhum cadastro. Confira o nome completo, o e-mail e a matrícula/SIAPE, ou procure a administração.")
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var id, nome, identificador, situacao string
	var temSenha bool
	var tentativas int
	var bloqueadoAte *time.Time
	err = tx.QueryRow(r.Context(), `SELECT u.id::text, u.nome_completo,
		COALESCE(NULLIF(u.identificador_institucional, ''), NULLIF(pa.matricula, ''), pd.siape, ''),
		u.situacao, u.hash_senha IS NOT NULL, u.tentativas_falhas, u.bloqueado_ate
		FROM usuarios u LEFT JOIN perfis_aluno pa ON pa.usuario_id = u.id LEFT JOIN perfis_docente pd ON pd.usuario_id = u.id
		WHERE u.email = $1 FOR UPDATE OF u`, normalizarEmail(req.Email)).
		Scan(&id, &nome, &identificador, &situacao, &temSenha, &tentativas, &bloqueadoAte)
	if errors.Is(err, pgx.ErrNoRows) {
		return naoConfere
	}
	if err != nil {
		return err
	}
	if bloqueadoAte != nil && bloqueadoAte.After(time.Now()) {
		return &ErroAPI{Status: http.StatusTooManyRequests, Codigo: "conta_bloqueada_temporariamente",
			Mensagem: "Muitas tentativas incorretas. Tente novamente em alguns minutos."}
	}
	confere := normalizarNome(req.Nome) == normalizarNome(nome) &&
		identificador != "" && normalizarIdentificador(req.Identificador) == normalizarIdentificador(identificador)
	if !confere {
		// Tentativas erradas contam para o bloqueio temporário (o mesmo do login).
		tentativas++
		var bloqueio any
		if tentativas >= maximoTentativas {
			bloqueio, tentativas = time.Now().Add(tempoBloqueio), 0
		}
		if _, err := tx.Exec(r.Context(), `UPDATE usuarios SET tentativas_falhas = $2, bloqueado_ate = $3 WHERE id = $1`, id, tentativas, bloqueio); err != nil {
			return err
		}
		if err := tx.Commit(r.Context()); err != nil {
			return err
		}
		if identificador == "" && normalizarNome(req.Nome) == normalizarNome(nome) {
			return erroRequisicao("Seu cadastro ainda não tem matrícula/SIAPE registrado. Peça à administração para completá-lo.")
		}
		return naoConfere
	}
	if situacao == "bloqueado" {
		return &ErroAPI{Status: http.StatusForbidden, Codigo: "conta_bloqueada", Mensagem: "Conta bloqueada. Procure a administração."}
	}
	if temSenha {
		return &ErroAPI{Status: http.StatusConflict, Codigo: "senha_ja_definida",
			Mensagem: "Esta conta já possui senha. Se você esqueceu a senha, peça à administração para redefinir o seu acesso."}
	}
	hash, err := seguranca.GerarHashSenha(req.Senha)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(r.Context(), `UPDATE usuarios SET hash_senha = $2, situacao = 'ativo', tentativas_falhas = 0, bloqueado_ate = NULL,
		atualizado_em = now() WHERE id = $1`, id, hash); err != nil {
		return err
	}
	if _, err := tx.Exec(r.Context(), `DELETE FROM sessoes WHERE usuario_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(r.Context(), `INSERT INTO registros_auditoria (ator_id, acao, entidade, entidade_id, ip)
		VALUES ($1, 'primeiro_acesso', 'usuario', $1, $2)`, id, r.RemoteAddr); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}

// ---------------------------------------------------------------------
// Minha conta
// ---------------------------------------------------------------------

type dadosConta struct {
	Nome          string `json:"nome"`
	Email         string `json:"email"`
	Telefone      string `json:"telefone"`
	Matricula     string `json:"matricula"`
	Curso         string `json:"curso"`
	SIAPE         string `json:"siape"`
	EhAluno       bool   `json:"ehAluno"`
	EhDocente     bool   `json:"ehDocente"`
}

func (s *Servidor) obterConta(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	var c dadosConta
	err := s.pool.QueryRow(r.Context(), `SELECT u.nome_completo, u.email, u.telefone,
		COALESCE(pa.matricula, ''), COALESCE(pa.curso, ''), COALESCE(pd.siape, ''), pa.usuario_id IS NOT NULL, pd.usuario_id IS NOT NULL
		FROM usuarios u LEFT JOIN perfis_aluno pa ON pa.usuario_id = u.id LEFT JOIN perfis_docente pd ON pd.usuario_id = u.id
		WHERE u.id = $1`, ses.UsuarioID).Scan(&c.Nome, &c.Email, &c.Telefone, &c.Matricula, &c.Curso, &c.SIAPE, &c.EhAluno, &c.EhDocente)
	if err != nil {
		return err
	}
	return responderOK(w, c)
}

func (s *Servidor) atualizarConta(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	var c dadosConta
	if err := lerJSON(r, &c); err != nil {
		return err
	}
	c.Nome = strings.Join(strings.Fields(c.Nome), " ")
	c.Telefone = strings.TrimSpace(c.Telefone)
	c.Matricula = strings.TrimSpace(c.Matricula)
	c.SIAPE = strings.TrimSpace(c.SIAPE)
	if len(c.Nome) < 3 {
		return erroCampo("nome", "Informe o nome completo.")
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	// Dados que constam em uma ATA já gerada não podem mudar sem revisão do processo.
	var possuiATA bool
	if err := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM atas a JOIN trabalhos t ON t.id = a.trabalho_id
		WHERE t.aluno_id = $1 AND a.estado <> 'cancelada')`, ses.UsuarioID).Scan(&possuiATA); err != nil {
		return err
	}
	if _, err := tx.Exec(r.Context(), `UPDATE usuarios SET telefone = $2, atualizado_em = now(),
		nome_completo = CASE WHEN $3 THEN nome_completo ELSE $4 END WHERE id = $1`, ses.UsuarioID, c.Telefone, possuiATA, c.Nome); err != nil {
		return err
	}
	if ses.TemPerfil(PerfilAluno) && !possuiATA {
		if c.Matricula == "" {
			return erroCampo("matricula", "Informe a matrícula.")
		}
		if _, err := tx.Exec(r.Context(), `UPDATE perfis_aluno SET matricula = $2, curso = COALESCE(NULLIF($3, ''), curso) WHERE usuario_id = $1`,
			ses.UsuarioID, c.Matricula, strings.TrimSpace(c.Curso)); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `UPDATE usuarios SET identificador_institucional = $2 WHERE id = $1`, ses.UsuarioID, c.Matricula); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(r.Context(), `UPDATE perfis_docente SET siape = $2 WHERE usuario_id = $1`, ses.UsuarioID, c.SIAPE); err != nil {
		return err
	}
	if c.SIAPE != "" {
		if _, err := tx.Exec(r.Context(), `UPDATE usuarios SET identificador_institucional = $2 WHERE id = $1 AND tipo_identificador = 'SIAPE'`,
			ses.UsuarioID, c.SIAPE); err != nil {
			return err
		}
	}
	if err := s.auditar(r.Context(), tx, r, "atualizar_conta", "usuario", ses.UsuarioID, nil); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	if possuiATA {
		return responderOK(w, map[string]any{"ok": true, "aviso": "Nome e matrícula não foram alterados porque já constam em uma ATA gerada."})
	}
	return responderOK(w, map[string]any{"ok": true})
}

func (s *Servidor) trocarSenha(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	var req struct {
		SenhaAtual string `json:"senhaAtual"`
		NovaSenha  string `json:"novaSenha"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	var hash *string
	if err := s.pool.QueryRow(r.Context(), `SELECT hash_senha FROM usuarios WHERE id = $1`, ses.UsuarioID).Scan(&hash); err != nil {
		return err
	}
	if hash == nil || !seguranca.VerificarSenha(req.SenhaAtual, *hash) {
		return erroCampo("senhaAtual", "Senha atual incorreta.")
	}
	if err := seguranca.ValidarForcaSenha(req.NovaSenha); err != nil {
		return erroCampo("novaSenha", "A nova senha deve ter pelo menos 8 caracteres.")
	}
	novo, err := seguranca.GerarHashSenha(req.NovaSenha)
	if err != nil {
		return err
	}
	if _, err := s.pool.Exec(r.Context(), `UPDATE usuarios SET hash_senha = $2, atualizado_em = now() WHERE id = $1`, ses.UsuarioID, novo); err != nil {
		return err
	}
	// Encerra as demais sessões do usuário.
	if _, err := s.pool.Exec(r.Context(), `DELETE FROM sessoes WHERE usuario_id = $1 AND id <> $2`, ses.UsuarioID, ses.ID); err != nil {
		return err
	}
	s.auditar(r.Context(), s.pool, r, "trocar_senha", "usuario", ses.UsuarioID, nil)
	return responderOK(w, map[string]bool{"ok": true})
}
