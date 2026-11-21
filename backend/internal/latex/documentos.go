package latex

import (
	"fmt"
	"strconv"
	"time"

	"wsi/internal/datas"
)

const (
	VersaoTemplateATA        = "ata-v3"
	VersaoTemplateDeclaracao = "declaracao-v2"
)

// LinhaCabecalho é uma linha do cabeçalho institucional.
type LinhaCabecalho struct {
	Texto   string
	Negrito bool
}

// Cabecalho padrão (seção 14.3 da especificação). Pode ser alterado pelo ADMIN
// em Configurações institucionais.
func CabecalhoPadrao() []LinhaCabecalho {
	return []LinhaCabecalho{
		{"MINISTÉRIO DA EDUCAÇÃO", true},
		{"SECRETARIA DE EDUCAÇÃO PROFISSIONAL E TECNOLÓGICA", true},
		{"INSTITUTO FEDERAL DE EDUCAÇÃO, CIÊNCIA E TECNOLOGIA DE MINAS GERAIS", true},
		{"CAMPUS OURO BRANCO", true},
		{"Av. Afonso Sardinha, nº 90, Bairro Pioneiros, CEP: 36420-000, Ouro Branco - Minas Gerais", false},
		{"(31) 3742-2149 - gabinete.ourobranco@ifmg.edu.br", false},
	}
}

func escaparCabecalho(linhas []LinhaCabecalho) []LinhaCabecalho {
	r := make([]LinhaCabecalho, 0, len(linhas))
	for _, l := range linhas {
		if t := Escapar(l.Texto); t != "" {
			r = append(r, LinhaCabecalho{Texto: t, Negrito: l.Negrito})
		}
	}
	return r
}

// ---------------------------------------------------------------------
// ATA
// ---------------------------------------------------------------------

// EntradaATA contém os dados crus (não escapados) da ATA.
type EntradaATA struct {
	Cabecalho          []LinhaCabecalho
	DataApresentacao   time.Time     // data da sessão/evento atual do trabalho
	HorarioApresentacao time.Duration // horário desde 00:00
	Local              string
	NomeAluno          string
	Matricula          string
	Titulo             string
	NomeOrientador     string
	Examinadores       []string // na ordem da banca
	NotaFinal          string   // já formatada: "87,00"
	Aprovado           bool     // nota final >= 60,00 (notas.Aprovado)
}

type AssinaturaATA struct {
	Rotulo string
	Nome   string
}

// DadosATA contém somente texto já escapado, pronto para o template.
type DadosATA struct {
	Cabecalho   []LinhaCabecalho
	Dia         string
	Mes         string
	Ano         string
	Horario     string
	Local       string
	NomeAluno   string
	Matricula   string
	Titulo      string
	NotaFinal   string
	Deliberacao string // "Aprovação" (nota >= 60) ou "Reprovação"
	ListaBanca  []string
	Assinaturas []AssinaturaATA
	// PaginasAssinaturas: blocos por página (página 2: até 4; seguintes: até 5).
	PaginasAssinaturas [][]AssinaturaATA
}

const (
	assinaturasPagina2      = 4
	assinaturasPorPaginaExtra = 5
)

func paginarAssinaturas(a []AssinaturaATA) [][]AssinaturaATA {
	var paginas [][]AssinaturaATA
	limite := assinaturasPagina2
	for len(a) > 0 {
		n := limite
		if n > len(a) {
			n = len(a)
		}
		paginas = append(paginas, a[:n])
		a = a[n:]
		limite = assinaturasPorPaginaExtra
	}
	return paginas
}

func (e EntradaATA) Validar() error {
	switch {
	case e.DataApresentacao.IsZero():
		return fmt.Errorf("data da apresentação ausente")
	case Escapar(e.Local) == "":
		return fmt.Errorf("local da apresentação ausente")
	case Escapar(e.NomeAluno) == "":
		return fmt.Errorf("nome do aluno ausente")
	case Escapar(e.Matricula) == "":
		return fmt.Errorf("matrícula do aluno ausente")
	case Escapar(e.Titulo) == "":
		return fmt.Errorf("título do trabalho ausente")
	case Escapar(e.NomeOrientador) == "":
		return fmt.Errorf("orientador ausente")
	case e.NotaFinal == "":
		return fmt.Errorf("nota final ausente")
	}
	return nil
}

// RotuloExaminador retorna "Examinador 1", "Examinador 2"...
func RotuloExaminador(i int) string { return "Examinador " + strconv.Itoa(i) }

const (
	DeliberacaoAprovacao  = "Aprovação"
	DeliberacaoReprovacao = "Reprovação"
	RotuloOrientador = "Professor Orientador"
	RotuloAluno      = "Aluno"
)

func (e EntradaATA) Preparar() (DadosATA, error) {
	if err := e.Validar(); err != nil {
		return DadosATA{}, err
	}
	d := DadosATA{
		Cabecalho: escaparCabecalho(e.Cabecalho),
		Dia:       datas.Dia(e.DataApresentacao),
		Mes:       datas.NomeMes(e.DataApresentacao.Month()),
		Ano:       strconv.Itoa(e.DataApresentacao.Year()),
		Horario:   datas.Horario(e.HorarioApresentacao),
		Local:     Escapar(e.Local),
		NomeAluno: Escapar(e.NomeAluno),
		Matricula: Escapar(e.Matricula),
		Titulo:    Escapar(e.Titulo),
		NotaFinal: Escapar(e.NotaFinal),
	}
	d.Deliberacao = DeliberacaoReprovacao
	if e.Aprovado {
		d.Deliberacao = DeliberacaoAprovacao
	}
	// Lista da primeira página: examinadores e, por último, o orientador (como no modelo).
	for i, ex := range e.Examinadores {
		d.ListaBanca = append(d.ListaBanca, fmt.Sprintf("%d - %s", i+1, Escapar(ex)))
	}
	d.ListaBanca = append(d.ListaBanca, fmt.Sprintf("%d - %s", len(e.Examinadores)+1, Escapar(e.NomeOrientador)))
	// Assinaturas: Professor Orientador, Examinador 1..N, Aluno.
	d.Assinaturas = append(d.Assinaturas, AssinaturaATA{RotuloOrientador, Escapar(e.NomeOrientador)})
	for i, ex := range e.Examinadores {
		d.Assinaturas = append(d.Assinaturas, AssinaturaATA{RotuloExaminador(i + 1), Escapar(ex)})
	}
	d.Assinaturas = append(d.Assinaturas, AssinaturaATA{RotuloAluno, Escapar(e.NomeAluno)})
	d.PaginasAssinaturas = paginarAssinaturas(d.Assinaturas)
	return d, nil
}

// ---------------------------------------------------------------------
// Declaração anti-plágio
// ---------------------------------------------------------------------

type EntradaDeclaracao struct {
	Cabecalho []LinhaCabecalho
	NomeAluno string
	Matricula string
	Cidade    string
	Data      time.Time // data de emissão
}

type DadosDeclaracao struct {
	Cabecalho []LinhaCabecalho
	NomeAluno string
	Matricula string
	Cidade    string
	Dia       string
	Mes       string
	Ano       string
}

func (e EntradaDeclaracao) Preparar() (DadosDeclaracao, error) {
	switch {
	case Escapar(e.NomeAluno) == "":
		return DadosDeclaracao{}, fmt.Errorf("nome do aluno ausente")
	case Escapar(e.Matricula) == "":
		return DadosDeclaracao{}, fmt.Errorf("matrícula ausente: complete seu cadastro antes de gerar a declaração")
	case Escapar(e.Cidade) == "":
		return DadosDeclaracao{}, fmt.Errorf("cidade não configurada")
	case e.Data.IsZero():
		return DadosDeclaracao{}, fmt.Errorf("data ausente")
	}
	return DadosDeclaracao{
		Cabecalho: escaparCabecalho(e.Cabecalho),
		NomeAluno: Escapar(e.NomeAluno),
		Matricula: Escapar(e.Matricula),
		Cidade:    Escapar(e.Cidade),
		Dia:       datas.Dia(e.Data),
		Mes:       datas.NomeMes(e.Data.Month()),
		Ano:       strconv.Itoa(e.Data.Year()),
	}, nil
}
