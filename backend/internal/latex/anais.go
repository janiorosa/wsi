package latex

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"wsi/internal/datas"
)

const VersaoTemplateAnaisAbertura = "anais-abertura-v1"

// Textos fixos das páginas iniciais dos anais (iguais em todas as edições).
const (
	TituloWorkshop   = "Workshop de Sistemas de Informação"
	InstituicaoAnais = "Instituto Federal de Minas Gerais"
	EstadoAnais      = "Minas Gerais"
	EnderecoAnais    = "Rua Afonso Sardinha, nº 90, Minas Talco, CEP: 36494-018"
)

// TrabalhoDestaque identifica um trabalho citado no editorial (premiado ou de maior nota).
type TrabalhoDestaque struct {
	Titulo     string
	Aluno      string
	Orientador string
}

// PremioAnais é uma premiação do evento com o(s) trabalho(s) mais votado(s) (empates: todos).
type PremioAnais struct {
	Nome       string
	Vencedores []TrabalhoDestaque
}

// EntradaAnaisAbertura contém os dados crus (não escapados) da capa, folha de rosto e editorial.
type EntradaAnaisAbertura struct {
	NumeroEdicao  int       // 6 => "VI Workshop", "v.6"
	Data          time.Time // dia do evento
	Cidade        string    // ex.: Ouro Branco
	Campus        string    // ex.: Ouro Branco
	Organizadores []string  // apresentadores do evento
	Premiacoes    []PremioAnais
	MaiorNota     []TrabalhoDestaque // trabalho(s) com a maior nota final (empates: todos)
}

// DadosAnaisAbertura contém somente texto já escapado (ou LaTeX montado a partir de texto escapado).
type DadosAnaisAbertura struct {
	LinhaVolume      string
	Instituicao      string
	TituloWorkshop   string
	LocalCapa        string
	DataLocal        string
	TituloFolhaRosto string
	Organizadores    []string
	Campus           string
	Endereco         string
	Paragrafos       []string
	Assinatura       string
	CargoAssinatura  string
}

var mesesAbreviados = [...]string{"jan.", "fev.", "mar.", "abr.", "mai.", "jun.", "jul.", "ago.", "set.", "out.", "nov.", "dez."}

// Romano converte 1..3999 em algarismos romanos.
func Romano(n int) string {
	if n <= 0 || n >= 4000 {
		return strconv.Itoa(n)
	}
	valores := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	simbolos := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}
	var b strings.Builder
	for i, v := range valores {
		for n >= v {
			b.WriteString(simbolos[i])
			n -= v
		}
	}
	return b.String()
}

// juntarLista: "A", "A e B", "A, B e C".
func juntarLista(itens []string) string {
	switch len(itens) {
	case 0:
		return ""
	case 1:
		return itens[0]
	}
	return strings.Join(itens[:len(itens)-1], ", ") + " e " + itens[len(itens)-1]
}

// descreverTrabalho: “Título”, de Aluno, sob orientação de Orientador (já escapado, título em negrito).
func descreverTrabalho(t TrabalhoDestaque) string {
	s := fmt.Sprintf(`\textbf{“%s”}, de %s`, Escapar(t.Titulo), Escapar(t.Aluno))
	if o := Escapar(t.Orientador); o != "" {
		s += ", sob orientação de " + o
	}
	return s
}

func (e EntradaAnaisAbertura) Preparar() (DadosAnaisAbertura, error) {
	switch {
	case e.NumeroEdicao <= 0:
		return DadosAnaisAbertura{}, fmt.Errorf("informe o número da edição do evento (ex.: 6 para o VI WSI) antes de gerar os anais")
	case e.Data.IsZero():
		return DadosAnaisAbertura{}, fmt.Errorf("data do evento ausente")
	case Escapar(e.Cidade) == "":
		return DadosAnaisAbertura{}, fmt.Errorf("cidade não configurada")
	}
	var organizadores []string
	for _, o := range e.Organizadores {
		if n := Escapar(o); n != "" {
			organizadores = append(organizadores, n)
		}
	}
	if len(organizadores) == 0 {
		return DadosAnaisAbertura{}, fmt.Errorf("o evento não possui apresentadores (organizadores) designados")
	}
	campus := Escapar(e.Campus)
	if campus == "" {
		campus = Escapar(e.Cidade)
	}
	romano := Romano(e.NumeroEdicao)
	ano := e.Data.Year()
	cidade := Escapar(e.Cidade)
	d := DadosAnaisAbertura{
		LinhaVolume:      fmt.Sprintf("v.%d, n.1, %s %d.", e.NumeroEdicao, mesesAbreviados[e.Data.Month()-1], ano),
		Instituicao:      InstituicaoAnais,
		TituloWorkshop:   strings.ToUpper(TituloWorkshop),
		LocalCapa:        cidade + ", " + EstadoAnais + ", Brasil",
		DataLocal:        fmt.Sprintf("%d de %s de %d, %s, %s", e.Data.Day(), datas.NomeMes(e.Data.Month()), ano, cidade, EstadoAnais),
		TituloFolhaRosto: "ANAIS DO " + romano + " " + strings.ToUpper(TituloWorkshop),
		Organizadores:    organizadores,
		Campus:           campus,
		Endereco:         EnderecoAnais,
		Assinatura:       juntarLista(organizadores),
	}
	cargo := "Coordenadores"
	if len(organizadores) == 1 {
		cargo = "Coordenador"
	}
	d.CargoAssinatura = fmt.Sprintf("%s do %s %s %d", cargo, romano, TituloWorkshop, ano)

	d.Paragrafos = append(d.Paragrafos, fmt.Sprintf(
		"O %dº %s (WSI), realizado em %d de %s de %d, teve como principal objetivo divulgar os trabalhos de conclusão do curso de Bacharelado em Sistemas de Informação do IFMG – Campus %s.",
		e.NumeroEdicao, TituloWorkshop, e.Data.Day(), strings.ToLower(datas.NomeMes(e.Data.Month())), ano, campus))

	// Premiações cadastradas para o evento e trabalhos vencedores (mais votados pelos docentes).
	var nomes []string
	for _, p := range e.Premiacoes {
		if n := Escapar(p.Nome); n != "" {
			nomes = append(nomes, `\textbf{`+n+`}`)
		}
	}
	if len(nomes) > 0 {
		var p string
		if len(nomes) == 1 {
			p = fmt.Sprintf("Com o objetivo de estimular e reconhecer a dedicação dos estudantes, o WSI %d promoveu o %s, que contemplou o melhor Trabalho de Conclusão de Curso (TCC) apresentado durante o evento.", ano, nomes[0])
		} else {
			p = fmt.Sprintf("Com o objetivo de estimular e reconhecer a dedicação dos estudantes, o WSI %d promoveu as premiações %s, que contemplaram os Trabalhos de Conclusão de Curso (TCC) de maior destaque apresentados durante o evento.", ano, juntarLista(nomes))
		}
		p += " A escolha foi realizada por meio de votação dos docentes participantes, considerando a originalidade, a complexidade, a aplicação e o impacto dos trabalhos na sociedade."
		for _, pr := range e.Premiacoes {
			if len(pr.Vencedores) == 0 || Escapar(pr.Nome) == "" {
				continue
			}
			var descricoes []string
			for _, v := range pr.Vencedores {
				descricoes = append(descricoes, descreverTrabalho(v))
			}
			if len(descricoes) == 1 {
				p += fmt.Sprintf(" A premiação \\textbf{%s} foi concedida ao trabalho %s.", Escapar(pr.Nome), descricoes[0])
			} else {
				p += fmt.Sprintf(" A premiação \\textbf{%s} foi concedida, em empate, aos trabalhos %s.", Escapar(pr.Nome), juntarLista(descricoes))
			}
		}
		d.Paragrafos = append(d.Paragrafos, p)
	}

	if len(e.MaiorNota) > 0 {
		var descricoes []string
		for _, t := range e.MaiorNota {
			descricoes = append(descricoes, descreverTrabalho(t))
		}
		if len(descricoes) == 1 {
			d.Paragrafos = append(d.Paragrafos, "Destacamos ainda o trabalho que obteve a maior nota final atribuída pelas bancas avaliadoras: "+descricoes[0]+".")
		} else {
			d.Paragrafos = append(d.Paragrafos, "Destacamos ainda os trabalhos que obtiveram, em empate, a maior nota final atribuída pelas bancas avaliadoras: "+juntarLista(descricoes)+".")
		}
	}

	d.Paragrafos = append(d.Paragrafos,
		"Expressamos nossos parabéns e agradecimentos aos estudantes que apresentaram seus trabalhos de elevada qualidade neste evento, o qual almejamos que se consolide como uma tradição em nossa comunidade. Espera-se que o WSI inspire diálogos profícuos, estimulando maior engajamento e interesse no desenvolvimento dos trabalhos dos estudantes, e que contribua significativamente para a área de Sistemas de Informação na resolução de problemas reais da comunidade.")
	return d, nil
}
