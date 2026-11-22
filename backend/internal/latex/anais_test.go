package latex

import (
	"context"
	"strings"
	"testing"
	"time"

	"wsi/internal/datas"
	"wsi/internal/pdf"
)

func TestRomano(t *testing.T) {
	for n, esperado := range map[int]string{1: "I", 4: "IV", 5: "V", 6: "VI", 9: "IX", 14: "XIV", 40: "XL", 2026: "MMXXVI"} {
		if r := Romano(n); r != esperado {
			t.Errorf("Romano(%d) = %q, esperado %q", n, r, esperado)
		}
	}
}

func entradaAnais(premiados, maiorNota int) EntradaAnaisAbertura {
	e := EntradaAnaisAbertura{
		NumeroEdicao:  6,
		Data:          time.Date(2026, time.July, 28, 0, 0, 0, 0, datas.Fuso),
		Cidade:        "Ouro Branco",
		Campus:        "Ouro Branco",
		Organizadores: []string{"Charles Tim Batista Garrocho", "Daniela Costa Terra"},
	}
	p := PremioAnais{Nome: "Prêmio Destaque: Reconhecendo a Excelência Acadêmica"}
	for i := 0; i < premiados; i++ {
		p.Vencedores = append(p.Vencedores, TrabalhoDestaque{Titulo: "Utilização de Jogos Interativos como Método de Ensino de Programação " + string(rune('A'+i)),
			Aluno: "Igor Guilherme Almeida Rocha", Orientador: "Charles Tim Batista Garrocho"})
	}
	e.Premiacoes = []PremioAnais{p}
	for i := 0; i < maiorNota; i++ {
		e.MaiorNota = append(e.MaiorNota, TrabalhoDestaque{Titulo: "Sistema de Planejamento Financeiro & 100% \"Planilhas\" " + string(rune('A'+i)),
			Aluno: "Otávio Henrique Dias Sena", Orientador: "Charles Tim Batista Garrocho"})
	}
	return e
}

func TestPrepararAnaisAbertura(t *testing.T) {
	d, err := entradaAnais(1, 1).Preparar()
	if err != nil {
		t.Fatal(err)
	}
	if d.LinhaVolume != "v.6, n.1, jul. 2026." {
		t.Fatalf("linha do volume incorreta: %q", d.LinhaVolume)
	}
	if d.TituloFolhaRosto != "ANAIS DO VI WORKSHOP DE SISTEMAS DE INFORMAÇÃO" {
		t.Fatalf("título da folha de rosto incorreto: %q", d.TituloFolhaRosto)
	}
	if d.DataLocal != "28 de Julho de 2026, Ouro Branco, Minas Gerais" {
		t.Fatalf("data/local incorretos: %q", d.DataLocal)
	}
	if d.Assinatura != "Charles Tim Batista Garrocho e Daniela Costa Terra" ||
		d.CargoAssinatura != "Coordenadores do VI Workshop de Sistemas de Informação 2026" {
		t.Fatalf("assinatura incorreta: %q / %q", d.Assinatura, d.CargoAssinatura)
	}
	editorial := strings.Join(d.Paragrafos, "\n")
	for _, esperado := range []string{"O 6º Workshop de Sistemas de Informação (WSI), realizado em 28 de julho de 2026",
		`A premiação \textbf{Prêmio Destaque: Reconhecendo a Excelência Acadêmica} foi concedida ao trabalho`,
		`100\% \textquotedbl{}Planilhas\textquotedbl{}`, `\&`, "maior nota final"} {
		if !strings.Contains(editorial, esperado) {
			t.Errorf("editorial sem o trecho %q:\n%s", esperado, editorial)
		}
	}
	// Sem número da edição ou sem organizadores, a geração é recusada com mensagem clara.
	e := entradaAnais(0, 0)
	e.NumeroEdicao = 0
	if _, err := e.Preparar(); err == nil {
		t.Fatal("sem número da edição a geração deveria ser recusada")
	}
	e = entradaAnais(0, 0)
	e.Organizadores = nil
	if _, err := e.Preparar(); err == nil {
		t.Fatal("sem organizadores a geração deveria ser recusada")
	}
	// Sem votos e sem notas: os parágrafos de destaque são omitidos.
	d, _ = entradaAnais(0, 0).Preparar()
	editorial = strings.Join(d.Paragrafos, "\n")
	if strings.Contains(editorial, "foi concedida") || strings.Contains(editorial, "maior nota final") {
		t.Fatalf("sem vencedores não deve haver destaque: %s", editorial)
	}
}

func TestCompilarAnaisAbertura(t *testing.T) {
	exigirFerramentas(t)
	casos := map[string]EntradaAnaisAbertura{"anais-abertura": entradaAnais(1, 1), "anais-abertura-empates": entradaAnais(3, 4)}
	for nome, e := range casos {
		d, err := e.Preparar()
		if err != nil {
			t.Fatal(err)
		}
		arquivo, res := compilar(t, DocumentoAnaisAbertura, d, nome)
		if res.Paginas < 3 {
			t.Fatalf("%s: esperadas ao menos 3 páginas, obtidas %d", nome, res.Paginas)
		}
		texto, _ := pdf.ExtrairTexto(context.Background(), arquivo)
		texto = normalizar(texto)
		for _, esperado := range []string{"v.6, n.1, jul. 2026.", "ANAIS", "WORKSHOP DE SISTEMAS DE INFORMAÇÃO", "Ouro Branco, Minas Gerais, Brasil",
			"INSTITUTO FEDERAL DE MINAS GERAIS", "28 de Julho de 2026, Ouro Branco, Minas Gerais", "ANAIS DO VI WORKSHOP DE SISTEMAS DE INFORMAÇÃO",
			"Organizadores", "Daniela Costa Terra", "Rua Afonso Sardinha, nº 90, Minas Talco, CEP: 36494-018", "EDITORIAL",
			"Coordenadores do VI Workshop de Sistemas de Informação 2026"} {
			if !strings.Contains(texto, esperado) {
				t.Errorf("%s: texto ausente: %q", nome, esperado)
			}
		}
	}
}
