package notas

import "testing"

func c(v float64) Centesimos {
	r, err := DeDecimal(v)
	if err != nil {
		panic(err)
	}
	return r
}

func TestCriterioUnicoEhANotaDoAvaliador(t *testing.T) {
	n, err := NotaAvaliador([]Centesimos{c(87.5)})
	if err != nil || n != 8750 {
		t.Fatalf("esperado 8750, obtido %d (%v)", n, err)
	}
}

func TestMediaSimplesSemPesosNemSoma(t *testing.T) {
	// 80 + 90 + 100 = 270 / 3 = 90 (não 270)
	n, _ := NotaAvaliador([]Centesimos{c(80), c(90), c(100)})
	if n != 9000 {
		t.Fatalf("esperado 9000, obtido %d", n)
	}
}

func TestArredondamentoMeioParaCima(t *testing.T) {
	// (90 + 85 + 85,01) / 3 = 86,67
	n, _ := MediaSimples([]Centesimos{9000, 8500, 8501})
	if n != 8667 {
		t.Fatalf("esperado 8667, obtido %d", n)
	}
	// (0,01 + 0,00) / 2 = 0,005 -> 0,01
	n, _ = MediaSimples([]Centesimos{1, 0})
	if n != 1 {
		t.Fatalf("esperado 1, obtido %d", n)
	}
	// (10 + 20 + 20) / 3 = 16,666... -> 16,67
	n, _ = MediaSimples([]Centesimos{1000, 2000, 2000})
	if n != 1667 {
		t.Fatalf("esperado 1667, obtido %d", n)
	}
}

func TestNotaTrabalhoMediaDosAvaliadoresIncluindoOrientador(t *testing.T) {
	orientador, _ := NotaAvaliador([]Centesimos{c(100), c(90)}) // 95
	ex1, _ := NotaAvaliador([]Centesimos{c(80), c(70)})         // 75
	ex2, _ := NotaAvaliador([]Centesimos{c(90), c(90)})         // 90
	final, _ := NotaTrabalho([]Centesimos{orientador, ex1, ex2})
	if final != 8667 { // 260/3 = 86,666..
		t.Fatalf("esperado 8667, obtido %d", final)
	}
	if final.FormatarBR() != "86,67" {
		t.Fatalf("formatação incorreta: %s", final.FormatarBR())
	}
}

func TestEscalaValidada(t *testing.T) {
	for _, v := range []float64{-0.01, 100.01, 150, -1} {
		if _, err := DeDecimal(v); err != ErrForaDaEscala {
			t.Fatalf("valor %v deveria ser rejeitado", v)
		}
	}
	if _, err := DeDecimal(87.555); err != ErrCasasDecimais {
		t.Fatal("três casas decimais deveriam ser rejeitadas")
	}
	for _, v := range []float64{0, 100, 0.1, 99.99, 33.3} {
		if _, err := DeDecimal(v); err != nil {
			t.Fatalf("valor %v deveria ser aceito: %v", v, err)
		}
	}
}

func TestFormatacoes(t *testing.T) {
	if Centesimos(10000).FormatarBR() != "100,00" || Centesimos(8700).FormatarBR() != "87,00" || Centesimos(5).FormatarBR() != "0,05" {
		t.Fatal("formatação BR incorreta")
	}
	v, err := DeTextoNumeric("87.5")
	if err != nil || v != 8750 {
		t.Fatalf("conversão numeric incorreta: %d %v", v, err)
	}
	v, _ = DeTextoNumeric("100.00")
	if v != 10000 {
		t.Fatal("conversão de 100.00 incorreta")
	}
}

func TestSemNotas(t *testing.T) {
	if _, err := MediaSimples(nil); err != ErrSemNotas {
		t.Fatal("lista vazia deveria gerar erro")
	}
}

func TestAprovacaoComSessentaOuMais(t *testing.T) {
	if !Aprovado(6000) || !Aprovado(10000) || !Aprovado(8333) {
		t.Fatal("notas >= 60,00 devem aprovar")
	}
	if Aprovado(5999) || Aprovado(0) {
		t.Fatal("notas < 60,00 não podem aprovar")
	}
}
