// Pacote datas concentra o fuso America/Sao_Paulo e a formatação de datas em português.
package datas

import (
	"fmt"
	"time"
)

var Fuso *time.Location

func init() {
	var err error
	Fuso, err = time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		// tzdata é instalado na imagem Docker; esta é apenas uma salvaguarda (UTC-3, sem horário de verão).
		Fuso = time.FixedZone("America/Sao_Paulo", -3*60*60)
	}
}

var meses = [...]string{"Janeiro", "Fevereiro", "Março", "Abril", "Maio", "Junho",
	"Julho", "Agosto", "Setembro", "Outubro", "Novembro", "Dezembro"}

// NomeMes retorna o nome do mês com inicial maiúscula, como nos modelos institucionais.
func NomeMes(m time.Month) string { return meses[m-1] }

// Agora retorna o instante atual no fuso de São Paulo.
func Agora() time.Time { return time.Now().In(Fuso) }

// Dia retorna o dia com dois dígitos, como nos modelos ("08", "14").
func Dia(t time.Time) string { return fmt.Sprintf("%02d", t.Day()) }

// PorExtenso retorna "14 de Julho de 2026".
func PorExtenso(t time.Time) string {
	return fmt.Sprintf("%s de %s de %d", Dia(t), NomeMes(t.Month()), t.Year())
}

// InicioDoDia retorna 00:00 da data informada no fuso de São Paulo.
func InicioDoDia(ano int, mes time.Month, dia int) time.Time {
	return time.Date(ano, mes, dia, 0, 0, 0, 0, Fuso)
}

// MomentoApresentacao combina data (do evento/sessão) e horário em um instante de São Paulo.
func MomentoApresentacao(data time.Time, horario time.Duration) time.Time {
	return time.Date(data.Year(), data.Month(), data.Day(), 0, 0, 0, 0, Fuso).Add(horario)
}

// Horario formata uma duração desde a meia-noite como "19:00".
func Horario(d time.Duration) string {
	minutos := int(d.Minutes())
	return fmt.Sprintf("%02d:%02d", minutos/60, minutos%60)
}
