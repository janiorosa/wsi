package latex

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"wsi/internal/pdf"
)

// Compilador preenche os templates e executa o XeLaTeX em diretório temporário isolado.
type Compilador struct {
	DiretorioTemplates string
	DiretorioTemp      string // base onde serão criados diretórios temporários
	TempoLimite        time.Duration
}

type Resultado struct {
	Paginas int
	Avisos  []string
}

type Documento struct {
	Arquivo        string // ex.: "ata/ata.tex.tmpl"
	PaginasMinimas int
	PaginasMaximas int      // 0 = sem limite
	Recursos       []string // imagens adicionais (relativas ao diretório de templates), copiadas com o nome base
}

var (
	DocumentoATA        = Documento{Arquivo: "ata/ata.tex.tmpl", PaginasMinimas: 2}
	DocumentoDeclaracao = Documento{Arquivo: "declaracao-antiplagio/declaracao.tex.tmpl", PaginasMinimas: 1, PaginasMaximas: 1}
	// Páginas iniciais dos anais: capa, folha de rosto e editorial (3 ou mais páginas).
	DocumentoAnaisAbertura = Documento{Arquivo: "anais/abertura.tex.tmpl", PaginasMinimas: 3,
		Recursos: []string{"anais/logo-wsi.png", "anais/logo-ifmg.png"}}
)

var (
	regexCaractereAusente = regexp.MustCompile(`Missing character: There is no (.+?) in font`)
	regexOverfull         = regexp.MustCompile(`Overfull \\hbox \(([\d.]+)pt too wide\)`)
	regexOverfullVertical = regexp.MustCompile(`Overfull \\vbox \(([\d.]+)pt too high\)`)
)

// Renderizar preenche o template e devolve o código LaTeX (útil para testes).
func (c *Compilador) Renderizar(doc Documento, dados any) ([]byte, error) {
	t, err := template.New(filepath.Base(doc.Arquivo)).Delims("<<", ">>").Option("missingkey=error").ParseFiles(
		filepath.Join(c.DiretorioTemplates, "comum", "preambulo.tex.tmpl"),
		filepath.Join(c.DiretorioTemplates, doc.Arquivo),
	)
	if err != nil {
		return nil, fmt.Errorf("erro ao carregar template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, filepath.Base(doc.Arquivo), dados); err != nil {
		return nil, fmt.Errorf("erro ao preencher template: %w", err)
	}
	return buf.Bytes(), nil
}

// Compilar gera o PDF em destino e valida o resultado:
// compilação sem erro, nenhum caractere ausente na fonte, A4 e número de páginas esperado.
func (c *Compilador) Compilar(ctx context.Context, doc Documento, dados any, destino string) (Resultado, error) {
	fonte, err := c.Renderizar(doc, dados)
	if err != nil {
		return Resultado{}, err
	}
	dir, err := os.MkdirTemp(c.DiretorioTemp, "latex-")
	if err != nil {
		return Resultado{}, err
	}
	defer os.RemoveAll(dir)

	if err := copiar(filepath.Join(c.DiretorioTemplates, "comum", "brasao.png"), filepath.Join(dir, "brasao.png")); err != nil {
		return Resultado{}, fmt.Errorf("brasão não encontrado: %w", err)
	}
	for _, recurso := range doc.Recursos {
		if err := copiar(filepath.Join(c.DiretorioTemplates, recurso), filepath.Join(dir, filepath.Base(recurso))); err != nil {
			return Resultado{}, fmt.Errorf("imagem do documento não encontrada (%s): %w", recurso, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "documento.tex"), fonte, 0o600); err != nil {
		return Resultado{}, err
	}

	limite := c.TempoLimite
	if limite == 0 {
		limite = 90 * time.Second
	}
	ctxCmd, cancel := context.WithTimeout(ctx, limite)
	defer cancel()
	cmd := exec.CommandContext(ctxCmd, "xelatex",
		"-no-shell-escape", "-interaction=nonstopmode", "-halt-on-error", "-file-line-error",
		"documento.tex")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME="+dir, "TEXMFOUTPUT="+dir, "openout_any=p", "shell_escape=f")
	saida, errCmd := cmd.CombinedOutput()
	logTex, _ := os.ReadFile(filepath.Join(dir, "documento.log"))
	if errCmd != nil {
		if errors.Is(ctxCmd.Err(), context.DeadlineExceeded) {
			return Resultado{}, fmt.Errorf("XeLaTeX excedeu o tempo limite de %s", limite)
		}
		return Resultado{}, fmt.Errorf("falha na compilação XeLaTeX: %s", trechoErro(string(logTex), string(saida)))
	}

	res := Resultado{}
	if m := regexCaractereAusente.FindAllStringSubmatch(string(logTex), -1); len(m) > 0 {
		var faltantes []string
		for _, x := range m {
			faltantes = append(faltantes, x[1])
		}
		return Resultado{}, fmt.Errorf("caracteres não suportados pela fonte do documento: %s", strings.Join(faltantes, ", "))
	}
	// Conteúdo mais alto que a página seria cortado no PDF: falha em vez de perder texto.
	for _, m := range regexOverfullVertical.FindAllStringSubmatch(string(logTex), -1) {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil && v > 2 {
			return Resultado{}, fmt.Errorf("o conteúdo excede a altura da página em %spt (texto seria cortado)", m[1])
		}
	}
	for _, m := range regexOverfull.FindAllStringSubmatch(string(logTex), -1) {
		res.Avisos = append(res.Avisos, "linha excede a margem em "+m[1]+"pt")
	}

	gerado := filepath.Join(dir, "documento.pdf")
	if err := pdf.Validar(ctx, gerado); err != nil {
		return Resultado{}, fmt.Errorf("PDF gerado inválido: %w", err)
	}
	if err := pdf.VerificarA4(ctx, gerado); err != nil {
		return Resultado{}, err
	}
	paginas, err := pdf.ContarPaginas(ctx, gerado)
	if err != nil {
		return Resultado{}, err
	}
	if paginas < doc.PaginasMinimas || (doc.PaginasMaximas > 0 && paginas > doc.PaginasMaximas) {
		return Resultado{}, fmt.Errorf("documento gerado com %d página(s), fora do esperado", paginas)
	}
	res.Paginas = paginas
	if err := copiar(gerado, destino); err != nil {
		return Resultado{}, err
	}
	return res, nil
}

func trechoErro(logTex, saida string) string {
	fonte := logTex
	if fonte == "" {
		fonte = saida
	}
	linhas := strings.Split(fonte, "\n")
	for i, l := range linhas {
		if strings.HasPrefix(l, "!") || strings.Contains(l, ".tex:") && strings.Contains(l, "error") {
			fim := i + 4
			if fim > len(linhas) {
				fim = len(linhas)
			}
			return strings.Join(linhas[i:fim], " | ")
		}
	}
	if len(fonte) > 500 {
		fonte = fonte[len(fonte)-500:]
	}
	return fonte
}

func copiar(origem, destino string) error {
	in, err := os.Open(origem)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(destino)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
