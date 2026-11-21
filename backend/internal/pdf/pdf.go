// Pacote pdf valida, inspeciona e junta arquivos PDF usando qpdf e poppler-utils,
// ambos instalados na imagem Docker do backend.
package pdf

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ErrNaoEhPDF = errors.New("o arquivo enviado não é um PDF válido")

const tempoLimite = 2 * time.Minute

// VerificarAssinaturaArquivo confere os bytes iniciais ("%PDF-"), sem confiar na extensão.
func VerificarAssinaturaArquivo(r io.ReaderAt) error {
	cabecalho := make([]byte, 1024)
	n, _ := r.ReadAt(cabecalho, 0)
	// A especificação PDF permite lixo antes de %PDF- nos primeiros 1024 bytes.
	if !bytes.Contains(cabecalho[:n], []byte("%PDF-")) {
		return ErrNaoEhPDF
	}
	return nil
}

// Validar confere a estrutura do PDF com "qpdf --check".
// Código 0 = válido; 3 = válido com avisos (comum em PDFs reais); 2 = erro.
func Validar(ctx context.Context, caminho string) error {
	f, err := os.Open(caminho)
	if err != nil {
		return err
	}
	err = VerificarAssinaturaArquivo(f)
	f.Close()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, tempoLimite)
	defer cancel()
	cmd := exec.CommandContext(ctx, "qpdf", "--check", caminho)
	saida, err := cmd.CombinedOutput()
	if err != nil {
		var errSaida *exec.ExitError
		if errors.As(err, &errSaida) && errSaida.ExitCode() == 3 {
			return nil
		}
		if strings.Contains(string(saida), "invalid password") {
			return fmt.Errorf("o PDF está protegido por senha; envie um arquivo sem senha de abertura")
		}
		return fmt.Errorf("%w (%s)", ErrNaoEhPDF, resumir(string(saida)))
	}
	return nil
}

// ContarPaginas usa "qpdf --show-npages".
func ContarPaginas(ctx context.Context, caminho string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, tempoLimite)
	defer cancel()
	saida, err := exec.CommandContext(ctx, "qpdf", "--show-npages", caminho).Output()
	if err != nil {
		var errSaida *exec.ExitError
		if !(errors.As(err, &errSaida) && errSaida.ExitCode() == 3) {
			return 0, fmt.Errorf("não foi possível contar as páginas: %w", err)
		}
	}
	return strconv.Atoi(strings.TrimSpace(string(saida)))
}

type DimensaoPagina struct {
	Pagina  int
	Largura float64 // pontos
	Altura  float64 // pontos
}

var regexTamanho = regexp.MustCompile(`^Page\s+(\d+)\s+size:\s+([\d.]+)\s+x\s+([\d.]+)\s+pts`)

// Dimensoes retorna o tamanho de cada página (via pdfinfo).
func Dimensoes(ctx context.Context, caminho string) ([]DimensaoPagina, error) {
	total, err := ContarPaginas(ctx, caminho)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, tempoLimite)
	defer cancel()
	saida, err := exec.CommandContext(ctx, "pdfinfo", "-f", "1", "-l", strconv.Itoa(total), caminho).Output()
	if err != nil {
		return nil, fmt.Errorf("pdfinfo falhou: %w", err)
	}
	var dims []DimensaoPagina
	sc := bufio.NewScanner(bytes.NewReader(saida))
	for sc.Scan() {
		m := regexTamanho.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		p, _ := strconv.Atoi(m[1])
		l, _ := strconv.ParseFloat(m[2], 64)
		a, _ := strconv.ParseFloat(m[3], 64)
		dims = append(dims, DimensaoPagina{Pagina: p, Largura: l, Altura: a})
	}
	if len(dims) != total {
		return nil, fmt.Errorf("pdfinfo retornou %d dimensões para %d páginas", len(dims), total)
	}
	return dims, nil
}

// VerificarA4 confere que todas as páginas são A4 retrato (595,28 x 841,89 pt, tolerância de 1 pt).
func VerificarA4(ctx context.Context, caminho string) error {
	dims, err := Dimensoes(ctx, caminho)
	if err != nil {
		return err
	}
	for _, d := range dims {
		if math.Abs(d.Largura-595.28) > 1 || math.Abs(d.Altura-841.89) > 1 {
			return fmt.Errorf("página %d não é A4 (%.1f x %.1f pt)", d.Pagina, d.Largura, d.Altura)
		}
	}
	return nil
}

// Juntar concatena os PDFs na ordem recebida, preservando tamanho e orientação de cada página.
func Juntar(ctx context.Context, entradas []string, saida string) error {
	if len(entradas) == 0 {
		return errors.New("nenhum arquivo para juntar")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	args := []string{"--empty", "--pages"}
	args = append(args, entradas...)
	args = append(args, "--", saida)
	out, err := exec.CommandContext(ctx, "qpdf", args...).CombinedOutput()
	if err != nil {
		var errSaida *exec.ExitError
		if errors.As(err, &errSaida) && errSaida.ExitCode() == 3 {
			return nil // concluído com avisos
		}
		return fmt.Errorf("falha ao juntar PDFs: %s", resumir(string(out)))
	}
	return nil
}

// RenderizarPNG gera imagens PNG das páginas (usado nos testes visuais).
func RenderizarPNG(ctx context.Context, caminho, prefixoSaida string, dpi int) error {
	ctx, cancel := context.WithTimeout(ctx, tempoLimite)
	defer cancel()
	out, err := exec.CommandContext(ctx, "pdftoppm", "-png", "-r", strconv.Itoa(dpi), caminho, prefixoSaida).CombinedOutput()
	if err != nil {
		return fmt.Errorf("pdftoppm falhou: %s", resumir(string(out)))
	}
	return nil
}

// ExtrairTexto retorna o texto do PDF (pdftotext), usado nos testes de conteúdo.
func ExtrairTexto(ctx context.Context, caminho string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, tempoLimite)
	defer cancel()
	out, err := exec.CommandContext(ctx, "pdftotext", "-layout", "-enc", "UTF-8", caminho, "-").Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext falhou: %w", err)
	}
	return string(out), nil
}

func resumir(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 300 {
		s = s[:300] + "..."
	}
	return s
}
