// Pacote documentos contém os processadores da fila: declaração, ATA,
// apresentações consolidadas e anais.
package documentos

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"wsi/internal/banco"
	"wsi/internal/latex"
)

// Chaves das configurações institucionais.
const (
	ChaveCabecalho = "cabecalho"
	ChaveCidade    = "cidade"
)

type LinhaCabecalhoJSON struct {
	Texto   string `json:"texto"`
	Negrito bool   `json:"negrito"`
}

// CabecalhoPadraoJSON é o valor inicial gravado pelo seed.
func CabecalhoPadraoJSON() string {
	var linhas []LinhaCabecalhoJSON
	for _, l := range latex.CabecalhoPadrao() {
		linhas = append(linhas, LinhaCabecalhoJSON{Texto: l.Texto, Negrito: l.Negrito})
	}
	b, _ := json.Marshal(linhas)
	return string(b)
}

// CarregarCabecalho lê o cabeçalho configurado; usa o padrão se ausente ou inválido.
func CarregarCabecalho(ctx context.Context, q banco.Executor) ([]latex.LinhaCabecalho, error) {
	var valor string
	err := q.QueryRow(ctx, `SELECT valor FROM configuracoes_institucionais WHERE chave = $1`, ChaveCabecalho).Scan(&valor)
	if errors.Is(err, pgx.ErrNoRows) {
		return latex.CabecalhoPadrao(), nil
	}
	if err != nil {
		return nil, err
	}
	var linhas []LinhaCabecalhoJSON
	if json.Unmarshal([]byte(valor), &linhas) != nil || len(linhas) == 0 {
		return latex.CabecalhoPadrao(), nil
	}
	r := make([]latex.LinhaCabecalho, 0, len(linhas))
	for _, l := range linhas {
		r = append(r, latex.LinhaCabecalho{Texto: l.Texto, Negrito: l.Negrito})
	}
	return r, nil
}

func CarregarCidade(ctx context.Context, q banco.Executor) (string, error) {
	var valor string
	err := q.QueryRow(ctx, `SELECT valor FROM configuracoes_institucionais WHERE chave = $1`, ChaveCidade).Scan(&valor)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && strings.TrimSpace(valor) == "") {
		return "Ouro Branco", nil
	}
	return valor, err
}
