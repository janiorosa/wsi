// Pacote api expõe a API REST /api/v1. Os arquivos estão organizados por módulo
// (autenticação, usuários, eventos, orientação, trabalhos, avaliações, atas,
// documentos, publicações, auditoria).
package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"wsi/internal/armazenamento"
	"wsi/internal/configuracao"
	"wsi/internal/fila"
)

type Servidor struct {
	cfg       configuracao.Configuracao
	pool      *pgxpool.Pool
	arm       *armazenamento.Armazenamento
	fila      *fila.Fila
	limitador *limitador
	openAPI   []byte
}

func Novo(cfg configuracao.Configuracao, pool *pgxpool.Pool, arm *armazenamento.Armazenamento, f *fila.Fila, openAPI []byte) *Servidor {
	return &Servidor{cfg: cfg, pool: pool, arm: arm, fila: f, limitador: novoLimitador(20, 5*time.Minute), openAPI: openAPI}
}

func (s *Servidor) Rotas() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(registrarRequisicao)
	r.Use(cabecalhosSeguranca)
	r.Use(s.cors)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/saude", s.h(s.saude))
		r.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
			w.Write(s.openAPI)
		})

		// Público (sem autenticação)
		r.Route("/publico", func(r chi.Router) {
			r.Get("/eventos", s.h(s.publicoListarEventos))
			r.Get("/eventos/{slug}", s.h(s.publicoEvento))
			r.Get("/eventos/{slug}/capa", s.h(s.publicoCapa))
			r.Get("/publicacoes/{id}/arquivo", s.h(s.publicoArquivoPublicacao))
		})

		r.Group(func(r chi.Router) {
			r.Use(s.carregarSessao)
			r.Use(s.verificarCSRF)

			r.Route("/auth", func(r chi.Router) {
				r.Get("/sessao", s.h(s.obterSessao))
				r.With(s.limitar).Post("/entrar", s.h(s.entrar))
				r.With(s.limitar).Post("/cadastro/aluno", s.h(s.cadastrarAluno))
				r.With(s.limitar).Post("/cadastro/docente", s.h(s.cadastrarDocente))
				r.With(s.limitar).Post("/primeiro-acesso", s.h(s.primeiroAcesso))
				r.Post("/sair", s.h(s.sair))
				r.With(s.autenticado).Post("/contexto", s.h(s.escolherContexto))
			})

			r.Route("/conta", func(r chi.Router) {
				r.Use(s.autenticado)
				r.Get("/", s.h(s.obterConta))
				r.Put("/", s.h(s.atualizarConta))
				r.With(s.limitar).Post("/senha", s.h(s.trocarSenha))
			})

			// Tarefas da fila: consulta de estado pelo solicitante.
			r.With(s.autenticado).Get("/tarefas/{id}", s.h(s.obterTarefa))

			// Downloads protegidos (autorização por objeto dentro de cada manipulador).
			r.With(s.autenticado).Get("/arquivos/{id}", s.h(s.baixarArquivoTrabalho))
			r.With(s.autenticado).Get("/documentos/{id}", s.h(s.baixarDocumentoGerado))
			r.With(s.autenticado).Get("/declaracoes-assinadas/{id}", s.h(s.baixarDeclaracaoAssinada))

			// ATA e assinaturas (aluno ou docente signatário).
			r.Route("/atas/{id}", func(r chi.Router) {
				r.Use(s.autenticado)
				r.Get("/", s.h(s.obterATA))
				r.Get("/documento", s.h(s.baixarDocumentoATA))
				r.Post("/assinatura", s.h(s.enviarAssinaturaATA))
			})

			r.Route("/admin", func(r chi.Router) {
				r.Use(s.exigirContexto(PerfilAdmin))
				r.Get("/painel", s.h(s.adminPainel))

				r.Get("/usuarios", s.h(s.adminListarUsuarios))
				r.Post("/usuarios", s.h(s.adminCriarUsuario))
				r.Get("/usuarios/{id}", s.h(s.adminObterUsuario))
				r.Patch("/usuarios/{id}", s.h(s.adminAtualizarUsuario))
				r.Post("/usuarios/{id}/perfis", s.h(s.adminAdicionarPerfil))
				r.Delete("/usuarios/{id}/perfis/{perfil}", s.h(s.adminRemoverPerfil))
				r.Post("/usuarios/{id}/redefinir-acesso", s.h(s.adminRedefinirAcesso))
				r.Post("/docentes/{id}/aprovar", s.h(s.adminAprovarDocente))
				r.Post("/docentes/{id}/rejeitar", s.h(s.adminRejeitarDocente))

				r.Get("/vinculos", s.h(s.adminListarVinculos))
				r.Post("/vinculos", s.h(s.adminCriarVinculo))
				r.Post("/vinculos/{id}/encerrar", s.h(s.adminEncerrarVinculo))

				r.Get("/eventos", s.h(s.adminListarEventos))
				r.Post("/eventos", s.h(s.adminCriarEvento))
				r.Get("/eventos/{id}", s.h(s.adminObterEvento))
				r.Put("/eventos/{id}", s.h(s.adminAtualizarEvento))
				r.Post("/eventos/{id}/capa", s.h(s.adminEnviarCapa))
				r.Post("/eventos/{id}/sessoes", s.h(s.adminCriarSessao))
				r.Put("/sessoes/{id}", s.h(s.adminAtualizarSessao))
				r.Delete("/sessoes/{id}", s.h(s.adminRemoverSessao))
				r.Post("/eventos/{id}/apresentadores", s.h(s.adminAdicionarApresentador))
				r.Delete("/eventos/{id}/apresentadores/{docenteId}", s.h(s.adminRemoverApresentador))
				r.Put("/eventos/{id}/programacao", s.h(s.definirProgramacao))
				r.Post("/eventos/{id}/premiacoes", s.h(s.adminCriarPremiacao))
				r.Put("/premiacoes/{id}", s.h(s.adminAtualizarPremiacao))
				r.Delete("/premiacoes/{id}", s.h(s.adminRemoverPremiacao))
				r.Get("/trabalhos", s.h(s.adminListarTrabalhos))
				r.Post("/trabalhos/{id}/evento", s.h(s.adminAssociarEventoExcecao))

				r.Get("/criterios", s.h(s.adminListarCriterios))
				r.Post("/criterios", s.h(s.adminCriarCriterio))
				r.Put("/criterios/{id}", s.h(s.adminAtualizarCriterio))

				r.Get("/configuracoes", s.h(s.adminObterConfiguracoes))
				r.Put("/configuracoes", s.h(s.adminSalvarConfiguracoes))

				r.Get("/tarefas", s.h(s.adminListarTarefas))
				r.Post("/tarefas/{id}/reprocessar", s.h(s.adminReprocessarTarefa))
				r.Get("/auditoria", s.h(s.adminListarAuditoria))
			})

			r.Route("/docente", func(r chi.Router) {
				r.Use(s.exigirContexto(PerfilDocente))
				r.Get("/painel", s.h(s.docentePainel))
				r.Get("/docentes", s.h(s.listarDocentesAprovados))
				r.Get("/eventos-futuros", s.h(s.docenteEventosFuturos))

				r.Get("/orientandos", s.h(s.docenteListarOrientandos))
				r.Get("/alunos-disponiveis", s.h(s.docenteAlunosDisponiveis))
				r.Post("/orientandos", s.h(s.docenteVincularAluno))
				r.Post("/orientandos/novo", s.h(s.docenteCadastrarAluno))
				r.Post("/vinculos/{id}/encerrar", s.h(s.docenteEncerrarVinculo))

				r.Get("/trabalhos/{id}", s.h(s.docenteFichaTrabalho))
				r.Put("/trabalhos/{id}/evento", s.h(s.docenteAssociarEvento))
				r.Put("/trabalhos/{id}/banca", s.h(s.docenteDefinirBanca))
				r.Post("/trabalhos/{id}/criterios", s.h(s.docenteCriarCriterioExtra))
				r.Delete("/trabalhos/{id}/criterios/{criterioId}", s.h(s.docenteRemoverCriterioExtra))
				r.Post("/trabalhos/{id}/avaliacoes/{avaliadorId}/reabrir", s.h(s.docenteReabrirAvaliacao))
				r.Post("/trabalhos/{id}/ata", s.h(s.docenteGerarATA))
				r.Post("/trabalhos/{id}/ata/reiniciar", s.h(s.docenteReiniciarATA))

				r.Get("/avaliacoes", s.h(s.docenteListarAvaliacoes))
				r.Get("/avaliacoes/{trabalhoId}", s.h(s.docenteObterAvaliacao))
				r.Put("/avaliacoes/{trabalhoId}", s.h(s.docenteSalvarAvaliacao))
				r.Put("/avaliacoes/{trabalhoId}/parecer", s.h(s.docenteSalvarParecer))

				r.Get("/premiacoes", s.h(s.docenteListarVotacoes))
				r.Post("/premiacoes/{id}/votos", s.h(s.docenteVotar))
				r.Delete("/premiacoes/{id}/votos/{trabalhoId}", s.h(s.docenteRemoverVoto))

				r.Get("/atas", s.h(s.docenteListarATAs))
			})

			r.Route("/apresentador", func(r chi.Router) {
				r.Use(s.exigirContexto(PerfilDocente))
				r.Get("/eventos", s.h(s.apresentadorListarEventos))
				r.Get("/eventos/{id}", s.h(s.apresentadorEvento))
				r.Put("/eventos/{id}/programacao", s.h(s.definirProgramacao))
				r.Post("/eventos/{id}/publicacoes", s.h(s.apresentadorGerarPublicacao))
				r.Get("/publicacoes/{id}/arquivo", s.h(s.apresentadorBaixarPublicacao))
				r.Post("/publicacoes/{id}/publicar", s.h(s.apresentadorPublicar))
			})

			r.Route("/aluno", func(r chi.Router) {
				r.Use(s.exigirContexto(PerfilAluno))
				r.Get("/painel", s.h(s.alunoPainel))
				r.Put("/trabalho", s.h(s.alunoSalvarTrabalho))
				r.Post("/trabalho/arquivos/{tipo}", s.h(s.alunoEnviarArquivo))
				r.Get("/docentes", s.h(s.listarDocentesAprovados))
				r.Post("/orientador", s.h(s.alunoEscolherOrientador))
				r.Post("/declaracao/gerar", s.h(s.alunoGerarDeclaracao))
				r.Post("/declaracao/assinada", s.h(s.alunoEnviarDeclaracaoAssinada))
			})
		})
	})
	return r
}

func (s *Servidor) saude(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := s.pool.Ping(ctx); err != nil {
		responderJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "erro", "banco": "indisponivel"})
		return nil
	}
	return responderOK(w, map[string]string{"status": "ok", "banco": "ok"})
}

func registrarRequisicao(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("requisição", "metodo", r.Method, "caminho", r.URL.Path, "status", ww.Status(),
			"duracao_ms", time.Since(inicio).Milliseconds(), "ip", r.RemoteAddr)
	})
}

func cabecalhosSeguranca(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// cors: por padrão o frontend é servido na mesma origem (proxy do nginx).
// Origens adicionais (ex.: desenvolvimento) devem ser listadas em WSI_ORIGENS_PERMITIDAS.
func (s *Servidor) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origem := r.Header.Get("Origin")
		if origem != "" {
			for _, permitida := range s.cfg.OrigensPermitidas {
				if origem == permitida {
					w.Header().Set("Access-Control-Allow-Origin", origem)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
					w.Header().Add("Vary", "Origin")
					break
				}
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Utilitário para os testes e para o main.
func ArquivoOpenAPI(caminho string) []byte {
	b, err := os.ReadFile(caminho)
	if err != nil {
		return []byte("# documentação OpenAPI indisponível\n")
	}
	return b
}
