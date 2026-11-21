// Pacote notas implementa a regra de cálculo das notas (seção 10.4 da especificação).
//
// Regra "media-simples-v1":
//   - cada critério (geral ou extra) recebe nota de 0 a 100, com no máximo 2 casas decimais;
//   - nota do avaliador = média aritmética simples das notas dos critérios do trabalho;
//   - nota do trabalho = média aritmética simples das notas (já arredondadas) dos avaliadores obrigatórios;
//   - não existem pesos;
//   - arredondamento: 2 casas decimais, "meio para cima" (0,005 -> 0,01).
//
// Toda a aritmética é feita em centésimos inteiros para evitar erros de ponto flutuante.
package notas

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const VersaoRegra = "media-simples-v1"

// Centesimos representa uma nota multiplicada por 100 (ex.: 87,50 -> 8750).
type Centesimos int64

const (
	NotaMinima Centesimos = 0
	NotaMaxima Centesimos = 10000
	// NotaMinimaAprovacao: nota final mínima (60,00) para aprovação do trabalho.
	NotaMinimaAprovacao Centesimos = 6000
)

// Aprovado indica se a nota final atinge o mínimo de aprovação (>= 60,00).
func Aprovado(notaFinal Centesimos) bool { return notaFinal >= NotaMinimaAprovacao }

var (
	ErrForaDaEscala   = errors.New("a nota deve estar entre 0 e 100")
	ErrCasasDecimais  = errors.New("a nota deve ter no máximo duas casas decimais")
	ErrSemNotas       = errors.New("não há notas para calcular a média")
)

// DeDecimal converte um número (vindo do JSON) para centésimos, validando escala e precisão.
func DeDecimal(valor float64) (Centesimos, error) {
	if math.IsNaN(valor) || math.IsInf(valor, 0) {
		return 0, ErrForaDaEscala
	}
	escalado := valor * 100
	arredondado := math.Round(escalado)
	if math.Abs(escalado-arredondado) > 1e-6 {
		return 0, ErrCasasDecimais
	}
	c := Centesimos(arredondado)
	if c < NotaMinima || c > NotaMaxima {
		return 0, ErrForaDaEscala
	}
	return c, nil
}

// MediaSimples calcula a média aritmética simples com arredondamento meio-para-cima.
func MediaSimples(valores []Centesimos) (Centesimos, error) {
	if len(valores) == 0 {
		return 0, ErrSemNotas
	}
	var soma int64
	for _, v := range valores {
		if v < NotaMinima || v > NotaMaxima {
			return 0, ErrForaDaEscala
		}
		soma += int64(v)
	}
	n := int64(len(valores))
	// Valores não negativos: (2*soma + n) / (2*n) arredonda meio-para-cima.
	return Centesimos((2*soma + n) / (2 * n)), nil
}

// NotaAvaliador: média simples das notas de todos os critérios aplicáveis ao trabalho.
func NotaAvaliador(notasCriterios []Centesimos) (Centesimos, error) {
	return MediaSimples(notasCriterios)
}

// NotaTrabalho: média simples das notas finais de todos os avaliadores obrigatórios.
func NotaTrabalho(notasAvaliadores []Centesimos) (Centesimos, error) {
	return MediaSimples(notasAvaliadores)
}

// FormatarBR formata com vírgula decimal e duas casas: 8750 -> "87,50".
func (c Centesimos) FormatarBR() string {
	return fmt.Sprintf("%d,%02d", int64(c)/100, int64(c)%100)
}

// Decimal retorna a representação "87.50" usada no PostgreSQL (numeric).
func (c Centesimos) Decimal() string {
	return fmt.Sprintf("%d.%02d", int64(c)/100, int64(c)%100)
}

// DeTextoNumeric converte o texto de uma coluna numeric(5,2) ("87.50") em centésimos.
func DeTextoNumeric(s string) (Centesimos, error) {
	s = strings.TrimSpace(s)
	inteira, fracao, _ := strings.Cut(s, ".")
	i, err := strconv.ParseInt(inteira, 10, 64)
	if err != nil {
		return 0, err
	}
	for len(fracao) < 2 {
		fracao += "0"
	}
	if len(fracao) > 2 {
		return 0, ErrCasasDecimais
	}
	f, err := strconv.ParseInt(fracao, 10, 64)
	if err != nil {
		return 0, err
	}
	return Centesimos(i*100 + f), nil
}

func (c Centesimos) Float() float64 { return float64(c) / 100 }
