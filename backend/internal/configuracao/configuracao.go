// Pacote configuracao lê as variáveis de ambiente do sistema.
package configuracao

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Configuracao struct {
	EnderecoHTTP         string
	URLBanco             string
	DiretorioDados       string
	DiretorioTemplates   string
	OrigensPermitidas    []string
	CookieSeguro         bool
	EmailAdminInicial    string
	ConcorrenciaFila     int
	TamanhoMaximoUpload  int64
	TempoLimiteLatexSegs int
}

func Carregar() (Configuracao, error) {
	c := Configuracao{
		EnderecoHTTP:         valor("WSI_ENDERECO_HTTP", ":8080"),
		URLBanco:             os.Getenv("WSI_URL_BANCO"),
		DiretorioDados:       valor("WSI_DIRETORIO_DADOS", "/dados"),
		DiretorioTemplates:   valor("WSI_DIRETORIO_TEMPLATES", "/app/templates/documentos"),
		CookieSeguro:         valor("WSI_COOKIE_SEGURO", "true") == "true",
		EmailAdminInicial:    strings.ToLower(valor("WSI_EMAIL_ADMIN_INICIAL", "janio.silva@ifmg.edu.br")),
		ConcorrenciaFila:     inteiro("WSI_CONCORRENCIA_FILA", 1),
		TamanhoMaximoUpload:  int64(inteiro("WSI_TAMANHO_MAXIMO_UPLOAD_MB", 30)) * 1024 * 1024,
		TempoLimiteLatexSegs: inteiro("WSI_TEMPO_LIMITE_LATEX_SEGUNDOS", 90),
	}
	if origens := os.Getenv("WSI_ORIGENS_PERMITIDAS"); origens != "" {
		for _, o := range strings.Split(origens, ",") {
			if o = strings.TrimSpace(o); o != "" {
				c.OrigensPermitidas = append(c.OrigensPermitidas, o)
			}
		}
	}
	if c.URLBanco == "" {
		return c, fmt.Errorf("variável WSI_URL_BANCO não definida")
	}
	// A especificação prevê no máximo duas gerações simultâneas.
	if c.ConcorrenciaFila < 1 {
		c.ConcorrenciaFila = 1
	}
	if c.ConcorrenciaFila > 2 {
		c.ConcorrenciaFila = 2
	}
	return c, nil
}

func valor(nome, padrao string) string {
	if v := strings.TrimSpace(os.Getenv(nome)); v != "" {
		return v
	}
	return padrao
}

func inteiro(nome string, padrao int) int {
	v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(nome)))
	if err != nil {
		return padrao
	}
	return v
}
