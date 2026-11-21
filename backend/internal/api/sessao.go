package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"wsi/internal/banco"
	"wsi/internal/seguranca"
)

const (
	PerfilAdmin   = "ADMIN"
	PerfilDocente = "DOCENTE"
	PerfilAluno   = "ALUNO"

	nomeCookieSessao = "wsi_sessao"
	duracaoSessao    = 12 * time.Hour
)

type Sessao struct {
	ID        string
	UsuarioID string
	Nome      string
	Email     string
	Contexto  string // "" enquanto o usuário ADMIN+DOCENTE não escolheu
	Perfis    []string
	CSRF      string
}

func (s *Sessao) TemPerfil(p string) bool { return slices.Contains(s.Perfis, p) }

type chaveContexto struct{}

func sessaoDe(r *http.Request) *Sessao {
	if s, ok := r.Context().Value(chaveContexto{}).(*Sessao); ok {
		return s
	}
	return nil
}

// carregarSessao lê o cookie e carrega sessão, usuário e perfis atuais do banco a cada requisição
// (assim a remoção de um perfil tem efeito imediato).
func (s *Servidor) carregarSessao(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(nomeCookieSessao)
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		var ses Sessao
		var contexto *string
		err = s.pool.QueryRow(r.Context(), `
			UPDATE sessoes SET ultimo_uso_em = now(), expira_em = now() + interval '12 hours'
			FROM usuarios u
			WHERE sessoes.hash_token = $1 AND sessoes.expira_em > now() AND u.id = sessoes.usuario_id AND u.situacao = 'ativo'
			RETURNING sessoes.id::text, u.id::text, u.nome_completo, u.email, sessoes.contexto, sessoes.token_csrf,
			  COALESCE((SELECT array_agg(perfil ORDER BY perfil) FROM perfis_usuario WHERE usuario_id = u.id), '{}'::text[])`,
			seguranca.HashToken(c.Value)).Scan(&ses.ID, &ses.UsuarioID, &ses.Nome, &ses.Email, &contexto, &ses.CSRF, &ses.Perfis)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				responderErro(w, r, err)
				return
			}
			apagarCookie(w, s.cfg.CookieSeguro)
			next.ServeHTTP(w, r)
			return
		}
		if contexto != nil && slices.Contains(ses.Perfis, *contexto) {
			ses.Contexto = *contexto
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), chaveContexto{}, &ses)))
	})
}

// verificarCSRF exige o cabeçalho X-CSRF-Token igual ao token da sessão em métodos que alteram estado.
func (s *Servidor) verificarCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if ses := sessaoDe(r); ses != nil {
			enviado := r.Header.Get("X-CSRF-Token")
			if subtle.ConstantTimeCompare([]byte(enviado), []byte(ses.CSRF)) != 1 {
				responderErro(w, r, &ErroAPI{Status: http.StatusForbidden, Codigo: "csrf", Mensagem: "Sessão desatualizada. Recarregue a página."})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Servidor) autenticado(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sessaoDe(r) == nil {
			responderErro(w, r, erroNaoAutenticado)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// exigirContexto garante sessão no contexto informado e perfil ainda presente no banco.
func (s *Servidor) exigirContexto(perfil string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ses := sessaoDe(r)
			if ses == nil {
				responderErro(w, r, erroNaoAutenticado)
				return
			}
			if ses.Contexto != perfil || !ses.TemPerfil(perfil) {
				responderErro(w, r, erroProibido("Esta área exige o contexto "+perfil+"."))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func definirCookie(w http.ResponseWriter, token string, seguro bool) {
	http.SetCookie(w, &http.Cookie{Name: nomeCookieSessao, Value: token, Path: "/", HttpOnly: true, Secure: seguro,
		SameSite: http.SameSiteLaxMode, MaxAge: int(duracaoSessao.Seconds())})
}

func apagarCookie(w http.ResponseWriter, seguro bool) {
	http.SetCookie(w, &http.Cookie{Name: nomeCookieSessao, Value: "", Path: "/", HttpOnly: true, Secure: seguro,
		SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

// ---------------------------------------------------------------------
// Limitação de taxa (proteção contra força bruta) — em memória, por IP.
// ---------------------------------------------------------------------

type limitador struct {
	mu      sync.Mutex
	maximo  int
	janela  time.Duration
	acessos map[string][]time.Time
}

func novoLimitador(maximo int, janela time.Duration) *limitador {
	return &limitador{maximo: maximo, janela: janela, acessos: map[string][]time.Time{}}
}

func (l *limitador) permitir(chave string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	agora := time.Now()
	recentes := l.acessos[chave][:0]
	for _, t := range l.acessos[chave] {
		if agora.Sub(t) < l.janela {
			recentes = append(recentes, t)
		}
	}
	if len(recentes) >= l.maximo {
		l.acessos[chave] = recentes
		return false
	}
	l.acessos[chave] = append(recentes, agora)
	if len(l.acessos) > 10000 { // limpeza simples
		for k, v := range l.acessos {
			if len(v) == 0 || agora.Sub(v[len(v)-1]) > l.janela {
				delete(l.acessos, k)
			}
		}
	}
	return true
}

func (s *Servidor) limitar(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.limitador.permitir(r.RemoteAddr + r.URL.Path) {
			responderErro(w, r, &ErroAPI{Status: http.StatusTooManyRequests, Codigo: "muitas_tentativas",
				Mensagem: "Muitas tentativas. Aguarde alguns minutos e tente novamente."})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------
// Auditoria
// ---------------------------------------------------------------------

// auditar registra operações sensíveis. Nunca incluir senhas, tokens ou segredos em dados.
func (s *Servidor) auditar(ctx context.Context, q banco.Executor, r *http.Request, acao, entidade, entidadeID string, dados map[string]any) error {
	var ator any
	if ses := sessaoDe(r); ses != nil {
		ator = ses.UsuarioID
		if dados == nil {
			dados = map[string]any{}
		}
		dados["contexto"] = ses.Contexto
	}
	if dados == nil {
		dados = map[string]any{}
	}
	b, err := json.Marshal(dados)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `INSERT INTO registros_auditoria (ator_id, acao, entidade, entidade_id, dados, ip)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6)`, ator, acao, entidade, entidadeID, string(b), r.RemoteAddr)
	return err
}
