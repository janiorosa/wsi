package datas

import (
	"testing"
	"time"
)

func TestPorExtenso(t *testing.T) {
	d := time.Date(2026, time.July, 8, 10, 0, 0, 0, Fuso)
	if PorExtenso(d) != "08 de Julho de 2026" {
		t.Fatalf("obtido %q", PorExtenso(d))
	}
	if NomeMes(time.March) != "Março" {
		t.Fatal("mês de março sem acento")
	}
}

func TestHorario(t *testing.T) {
	if Horario(19*time.Hour) != "19:00" || Horario(9*time.Hour+5*time.Minute) != "09:05" {
		t.Fatal("formatação de horário incorreta")
	}
}

func TestMomentoApresentacaoUsaFusoDeSaoPaulo(t *testing.T) {
	m := MomentoApresentacao(time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC), 19*time.Hour)
	if m.Hour() != 19 || m.Location() != Fuso {
		t.Fatalf("momento incorreto: %v", m)
	}
}
