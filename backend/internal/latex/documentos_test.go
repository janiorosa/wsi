package latex

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"wsi/internal/datas"
	"wsi/internal/pdf"
)

func diretorioTemplates(t *testing.T) string {
	_, arquivo, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(arquivo), "..", "..", "templates", "documentos")
}

func entradaATA(examinadores int) EntradaATA {
	e := EntradaATA{
		Cabecalho:           CabecalhoPadrao(),
		DataApresentacao:    time.Date(2026, time.July, 14, 0, 0, 0, 0, datas.Fuso),
		HorarioApresentacao: 19 * time.Hour,
		Local:               "Salão de Convenções do Campus IFMG Ouro Branco",
		NomeAluno:           "Aluno de Teste da Silva",
		Matricula:           "0000000",
		Titulo:              "Título de Teste para Conferência do Layout",
		NomeOrientador:      "Orientador de Teste",
		NotaFinal:           "87,00",
		Aprovado:            true,
	}
	for i := 1; i <= examinadores; i++ {
		e.Examinadores = append(e.Examinadores, "Examinador de Teste Número "+string(rune('0'+i)))
	}
	return e
}

func TestPrepararATARotulosEOrdem(t *testing.T) {
	d, err := entradaATA(2).Preparar()
	if err != nil {
		t.Fatal(err)
	}
	if d.Dia != "14" || d.Mes != "Julho" || d.Ano != "2026" || d.Horario != "19:00" {
		t.Fatalf("data/horário incorretos: %+v", d)
	}
	rotulos := []string{}
	for _, a := range d.Assinaturas {
		rotulos = append(rotulos, a.Rotulo)
	}
	if strings.Join(rotulos, "|") != "Professor Orientador|Examinador 1|Examinador 2|Aluno" {
		t.Fatalf("rótulos incorretos: %v", rotulos)
	}
	if d.ListaBanca[2] != "3 - Orientador de Teste" {
		t.Fatalf("orientador deveria ser o último da lista da página 1: %v", d.ListaBanca)
	}
}

func TestPrepararATAExigeDados(t *testing.T) {
	e := entradaATA(2)
	e.Matricula = "   "
	if _, err := e.Preparar(); err == nil {
		t.Fatal("matrícula vazia deveria impedir a geração")
	}
	e = entradaATA(2)
	e.DataApresentacao = time.Time{}
	if _, err := e.Preparar(); err == nil {
		t.Fatal("data ausente deveria impedir a geração")
	}
}

func TestRenderizarATAEscapaTextoDoUsuario(t *testing.T) {
	c := &Compilador{DiretorioTemplates: diretorioTemplates(t)}
	e := entradaATA(2)
	e.Titulo = `Uso de 100% de $dados$ & \input{/etc/passwd} #1`
	d, err := e.Preparar()
	if err != nil {
		t.Fatal(err)
	}
	fonte, err := c.Renderizar(DocumentoATA, d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(fonte)
	if strings.Contains(s, `\input{/etc/passwd}`) {
		t.Fatal("comando LaTeX do usuário não foi escapado")
	}
	if !strings.Contains(s, `100\% de \$dados\$ \& \textbackslash{}input\{/etc/passwd\} \#1`) {
		t.Fatal("título escapado não encontrado no código gerado")
	}
	for _, esperado := range []string{"ANEXO II – ATA DE CONCLUSÃO DE TCC", `\textbf{\uline{X}}`, "Professor Orientador: Orientador de Teste",
		"Examinador 2: Examinador de Teste Número 2", "Aluno: Aluno de Teste da Silva", "MINISTÉRIO DA EDUCAÇÃO",
		"CEP: 36420-000", `deliberou pela \textbf{\uline{Aprovação}}`} {
		if !strings.Contains(s, esperado) {
			t.Errorf("trecho esperado ausente: %q", esperado)
		}
	}
	if strings.Contains(s, "<<") || strings.Contains(s, ">>") {
		t.Fatal("restaram marcadores de template não preenchidos")
	}
}

func TestRenderizarDeclaracaoPreservaTextoJuridico(t *testing.T) {
	c := &Compilador{DiretorioTemplates: diretorioTemplates(t)}
	d, err := EntradaDeclaracao{Cabecalho: CabecalhoPadrao(), NomeAluno: "Fulana de Tal", Matricula: "0100789", Cidade: "Ouro Branco",
		Data: time.Date(2026, 7, 8, 12, 0, 0, 0, datas.Fuso)}.Preparar()
	if err != nil {
		t.Fatal(err)
	}
	fonte, err := c.Renderizar(DocumentoDeclaracao, d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(fonte)
	for _, esperado := range []string{
		"DECLARAÇÃO ANTI-PLÁGIO",
		"sob as penalidades previstas no art. 299 do Código Penal Brasileiro",
		"Pena – reclusão, de 1 (um) a 5 (cinco) anos",
		"aumenta-se a pena de sexta parte”.",
		"Este crime engloba plágio e compra fraudulenta de documentos científicos.",
		`Ouro Branco, \textbf{\uline{08}} de \textbf{\uline{Julho}} de \textbf{\uline{2026}}`,
		`\textbf{Aluno: Fulana de Tal}`,
	} {
		if !strings.Contains(s, esperado) {
			t.Errorf("trecho esperado ausente: %q", esperado)
		}
	}
}

// ---------------------------------------------------------------------
// Testes de compilação real (executados no container de testes, que possui XeLaTeX).
// Os PDFs e PNGs gerados ficam em WSI_DIRETORIO_TESTES_VISUAIS para conferência visual.
// ---------------------------------------------------------------------

func exigirFerramentas(t *testing.T) {
	for _, f := range []string{"xelatex", "qpdf", "pdfinfo", "pdftotext"} {
		if _, err := exec.LookPath(f); err != nil {
			t.Skipf("%s não disponível; teste de compilação executado apenas no container de testes", f)
		}
	}
}

func diretorioSaidaVisual(t *testing.T) string {
	if d := os.Getenv("WSI_DIRETORIO_TESTES_VISUAIS"); d != "" {
		os.MkdirAll(d, 0o755)
		return d
	}
	return t.TempDir()
}

func compilar(t *testing.T, doc Documento, dados any, nome string) (string, Resultado) {
	t.Helper()
	c := &Compilador{DiretorioTemplates: diretorioTemplates(t), DiretorioTemp: t.TempDir(), TempoLimite: 2 * time.Minute}
	saida := filepath.Join(diretorioSaidaVisual(t), nome+".pdf")
	res, err := c.Compilar(context.Background(), doc, dados, saida)
	if err != nil {
		t.Fatalf("%s: %v", nome, err)
	}
	if _, err := exec.LookPath("pdftoppm"); err == nil {
		pdf.RenderizarPNG(context.Background(), saida, strings.TrimSuffix(saida, ".pdf"), 60)
	}
	return saida, res
}

func normalizar(s string) string {
	return regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
}

func TestCompilarATAComDiferentesBancas(t *testing.T) {
	exigirFerramentas(t)
	for _, n := range []int{1, 2, 3, 6} {
		e := entradaATA(n)
		d, err := e.Preparar()
		if err != nil {
			t.Fatal(err)
		}
		arquivo, res := compilar(t, DocumentoATA, d, "ata-"+string(rune('0'+n))+"-examinadores")
		if len(res.Avisos) > 0 {
			t.Errorf("banca com %d examinadores gerou avisos de margem: %v", n, res.Avisos)
		}
		// Página 2: até 4 assinaturas; demais páginas: até 5.
		esperadas := 2
		if extra := n + 2 - 4; extra > 0 {
			esperadas += (extra + 4) / 5
		}
		if res.Paginas != esperadas {
			t.Errorf("com %d examinador(es) a ATA deveria ter %d páginas, tem %d", n, esperadas, res.Paginas)
		}
		texto, err := pdf.ExtrairTexto(context.Background(), arquivo)
		if err != nil {
			t.Fatal(err)
		}
		texto = normalizar(texto)
		for _, esperado := range []string{"Aos 14 dias do mês de Julho de 2026, às 19:00 horas", "Professor Orientador: Orientador de Teste",
			"Aluno: Aluno de Teste da Silva", "atribuindo a nota 87,00", "Salão de Convenções do Campus IFMG Ouro Branco"} {
			if !strings.Contains(texto, esperado) {
				t.Errorf("ATA com %d examinadores: texto ausente %q", n, esperado)
			}
		}
		// Cabeçalho repetido em todas as páginas.
		if c := strings.Count(texto, "MINISTÉRIO DA EDUCAÇÃO"); c != res.Paginas {
			t.Errorf("cabeçalho aparece %d vezes em %d páginas", c, res.Paginas)
		}
		for i := 1; i <= n; i++ {
			if !strings.Contains(texto, "Examinador "+string(rune('0'+i))+": ") {
				t.Errorf("assinatura do examinador %d ausente", i)
			}
		}
	}
}

func TestCompilarATAComTextosLongosEEspeciais(t *testing.T) {
	exigirFerramentas(t)
	e := entradaATA(3)
	e.NomeAluno = "Maria Aparecida dos Santos Gonçalves de Albuquerque e Vasconcelos Júnior"
	e.Titulo = "Uma Abordagem Baseada em Aprendizado de Máquina & Teoria de Resposta ao Item (TRI) para Predição de 100% do Desempenho no ENEM: Estudo de Caso com Dados_Abertos {2019–2024} e Comparação com Métodos Clássicos #1"
	e.Local = "Salão de Convenções do Campus IFMG Ouro Branco – Bloco B"
	d, err := e.Preparar()
	if err != nil {
		t.Fatal(err)
	}
	arquivo, res := compilar(t, DocumentoATA, d, "ata-textos-longos")
	if len(res.Avisos) > 0 {
		t.Errorf("textos longos geraram avisos de margem: %v", res.Avisos)
	}
	texto, _ := pdf.ExtrairTexto(context.Background(), arquivo)
	texto = normalizar(texto)
	for _, esperado := range []string{"100%", "&", "#1", "Dados_Abertos", "{2019–2024}", "Albuquerque e Vasconcelos Júnior"} {
		if !strings.Contains(texto, esperado) {
			t.Errorf("texto especial ausente ou cortado: %q", esperado)
		}
	}
}

func TestCompilarDeclaracao(t *testing.T) {
	exigirFerramentas(t)
	d, err := EntradaDeclaracao{Cabecalho: CabecalhoPadrao(), NomeAluno: "Fabrício Augusto Miranda dos Santos", Matricula: "0039952",
		Cidade: "Ouro Branco", Data: time.Date(2026, 7, 8, 12, 0, 0, 0, datas.Fuso)}.Preparar()
	if err != nil {
		t.Fatal(err)
	}
	arquivo, res := compilar(t, DocumentoDeclaracao, d, "declaracao")
	if res.Paginas != 1 {
		t.Fatalf("declaração deveria ter 1 página, tem %d", res.Paginas)
	}
	texto, _ := pdf.ExtrairTexto(context.Background(), arquivo)
	texto = normalizar(texto)
	for _, esperado := range []string{"DECLARAÇÃO ANTI-PLÁGIO", "Eu, Fabrício Augusto Miranda dos Santos, matrícula 0039952",
		"Ouro Branco, 08 de Julho de 2026", "Aluno: Fabrício Augusto Miranda dos Santos"} {
		if !strings.Contains(texto, esperado) {
			t.Errorf("texto ausente: %q", esperado)
		}
	}
}

func TestCaractereForaDaFonteFalha(t *testing.T) {
	exigirFerramentas(t)
	e := entradaATA(2)
	e.Titulo = "Título com ideograma 漢字"
	d, _ := e.Preparar()
	c := &Compilador{DiretorioTemplates: diretorioTemplates(t), DiretorioTemp: t.TempDir()}
	_, err := c.Compilar(context.Background(), DocumentoATA, d, filepath.Join(t.TempDir(), "x.pdf"))
	if err == nil || !strings.Contains(err.Error(), "caracteres não suportados") {
		t.Fatalf("esperado erro de caractere não suportado, obtido %v", err)
	}
}

func TestDeliberacaoConformeNotaMinima(t *testing.T) {
	e := entradaATA(2)
	e.NotaFinal, e.Aprovado = "59,99", false
	d, err := e.Preparar()
	if err != nil {
		t.Fatal(err)
	}
	if d.Deliberacao != "Reprovação" {
		t.Fatalf("nota abaixo de 60 deveria gerar Reprovação, obtido %q", d.Deliberacao)
	}
	e.NotaFinal, e.Aprovado = "60,00", true
	d, _ = e.Preparar()
	if d.Deliberacao != "Aprovação" {
		t.Fatalf("nota 60 deveria gerar Aprovação, obtido %q", d.Deliberacao)
	}
}

func TestPaginarAssinaturas(t *testing.T) {
	for _, caso := range []struct{ total, paginas, ultima int }{{2, 1, 2}, {4, 1, 4}, {5, 2, 1}, {8, 2, 4}, {9, 2, 5}, {10, 3, 1}} {
		a := make([]AssinaturaATA, caso.total)
		p := paginarAssinaturas(a)
		if len(p) != caso.paginas || len(p[len(p)-1]) != caso.ultima {
			t.Errorf("%d assinaturas: %d páginas (última com %d)", caso.total, len(p), len(p[len(p)-1]))
		}
	}
}
