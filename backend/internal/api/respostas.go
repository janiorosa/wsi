package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErroAPI é o formato único de erro: {"erro": {"codigo", "mensagem", "campos"}}.
type ErroAPI struct {
	Status   int               `json:"-"`
	Codigo   string            `json:"codigo"`
	Mensagem string            `json:"mensagem"`
	Campos   map[string]string `json:"campos,omitempty"`
}

func (e *ErroAPI) Error() string { return e.Mensagem }

func erroValidacao(mensagem string, campos map[string]string) *ErroAPI {
	return &ErroAPI{Status: http.StatusBadRequest, Codigo: "validacao", Mensagem: mensagem, Campos: campos}
}

func erroCampo(campo, mensagem string) *ErroAPI {
	return erroValidacao(mensagem, map[string]string{campo: mensagem})
}

func erroRequisicao(mensagem string) *ErroAPI {
	return &ErroAPI{Status: http.StatusBadRequest, Codigo: "requisicao_invalida", Mensagem: mensagem}
}

func erroConflito(mensagem string) *ErroAPI {
	return &ErroAPI{Status: http.StatusConflict, Codigo: "conflito", Mensagem: mensagem}
}

func erroProibido(mensagem string) *ErroAPI {
	if mensagem == "" {
		mensagem = "Você não tem permissão para esta ação."
	}
	return &ErroAPI{Status: http.StatusForbidden, Codigo: "proibido", Mensagem: mensagem}
}

func erroNaoEncontrado(mensagem string) *ErroAPI {
	if mensagem == "" {
		mensagem = "Registro não encontrado."
	}
	return &ErroAPI{Status: http.StatusNotFound, Codigo: "nao_encontrado", Mensagem: mensagem}
}

var erroNaoAutenticado = &ErroAPI{Status: http.StatusUnauthorized, Codigo: "nao_autenticado", Mensagem: "Faça login para continuar."}

type manipulador func(w http.ResponseWriter, r *http.Request) error

// h adapta um manipulador que devolve erro para http.HandlerFunc.
func (s *Servidor) h(f manipulador) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			responderErro(w, r, err)
		}
	}
}

func responderJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if v != nil {
		json.NewEncoder(w).Encode(v)
	}
}

func responderOK(w http.ResponseWriter, v any) error {
	responderJSON(w, http.StatusOK, v)
	return nil
}

func responderErro(w http.ResponseWriter, r *http.Request, err error) {
	var e *ErroAPI
	if errors.As(err, &e) {
		responderJSON(w, e.Status, map[string]any{"erro": e})
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		responderJSON(w, http.StatusNotFound, map[string]any{"erro": erroNaoEncontrado("")})
		return
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			responderJSON(w, http.StatusConflict, map[string]any{"erro": erroConflito(mensagemUnicidade(pgErr.ConstraintName))})
			return
		case "23514", "23503", "22P02", "22007", "22008":
			msg := "Dados inválidos."
			if pgErr.Code == "23514" && strings.Contains(pgErr.Message, "ALUNO") {
				msg = "O perfil ALUNO é exclusivo e não pode ser combinado com outros perfis."
			}
			responderJSON(w, http.StatusBadRequest, map[string]any{"erro": erroRequisicao(msg)})
			return
		}
	}
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		responderJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"erro": &ErroAPI{Codigo: "arquivo_grande",
			Mensagem: fmt.Sprintf("Arquivo maior que o limite permitido (%d MB).", maxBytes.Limit/1024/1024)}})
		return
	}
	slog.Error("erro interno", "metodo", r.Method, "caminho", r.URL.Path, "erro", err)
	responderJSON(w, http.StatusInternalServerError, map[string]any{"erro": &ErroAPI{Codigo: "erro_interno",
		Mensagem: "Ocorreu um erro inesperado. Tente novamente em instantes."}})
}

func mensagemUnicidade(restricao string) string {
	switch restricao {
	case "usuarios_email_unico":
		return "Já existe uma conta com este e-mail."
	case "perfis_aluno_matricula_unica":
		return "Já existe um aluno com esta matrícula."
	case "vinculos_orientacao_ativo_unico":
		return "O aluno já possui um orientador ativo."
	case "trabalhos_aluno_ativo_unico":
		return "O aluno já possui um trabalho em andamento."
	case "membros_banca_docente_unico":
		return "O docente já faz parte desta banca."
	case "atas_ativa_unica":
		return "Já existe uma ATA ativa para este trabalho."
	case "publicacoes_evento_publicada_unica":
		return "Já existe uma publicação deste tipo em processo de publicação."
	case "eventos_slug_key":
		return "Já existe um evento com este endereço público."
	}
	return "Registro duplicado."
}

const tamanhoMaximoJSON = 1 << 20

func lerJSON(r *http.Request, destino any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, tamanhoMaximoJSON))
	if err := dec.Decode(destino); err != nil {
		if errors.Is(err, io.EOF) {
			return erroRequisicao("Corpo da requisição vazio.")
		}
		return erroRequisicao("JSON inválido.")
	}
	return nil
}

// Paginação: ?pagina=1&tamanho=20 (máximo 100).
type paginacao struct {
	Pagina  int
	Tamanho int
}

func (p paginacao) Offset() int { return (p.Pagina - 1) * p.Tamanho }

func lerPaginacao(r *http.Request) paginacao {
	p := paginacao{Pagina: 1, Tamanho: 20}
	if v, err := strconv.Atoi(r.URL.Query().Get("pagina")); err == nil && v > 0 {
		p.Pagina = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("tamanho")); err == nil && v > 0 {
		p.Tamanho = v
	}
	if p.Tamanho > 100 {
		p.Tamanho = 100
	}
	return p
}

type paginaResposta[T any] struct {
	Itens   []T `json:"itens"`
	Total   int `json:"total"`
	Pagina  int `json:"pagina"`
	Tamanho int `json:"tamanho"`
}

func novaPagina[T any](itens []T, total int, p paginacao) paginaResposta[T] {
	if itens == nil {
		itens = []T{}
	}
	return paginaResposta[T]{Itens: itens, Total: total, Pagina: p.Pagina, Tamanho: p.Tamanho}
}

// ordenacao valida o parâmetro ?ordem= contra uma lista branca de colunas.
func ordenacao(r *http.Request, permitidas map[string]string, padrao string) string {
	campo := r.URL.Query().Get("ordem")
	direcao := "ASC"
	if strings.HasPrefix(campo, "-") {
		direcao = "DESC"
		campo = campo[1:]
	}
	if coluna, ok := permitidas[campo]; ok {
		return coluna + " " + direcao
	}
	return padrao
}

func vazio(s string) bool { return strings.TrimSpace(s) == "" }

func textoOuNil(s string) any {
	if vazio(s) {
		return nil
	}
	return strings.TrimSpace(s)
}
