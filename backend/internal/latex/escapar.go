// Pacote latex prepara textos e compila documentos com XeLaTeX.
package latex

import (
	"strings"
	"unicode"
)

// Escapar converte texto livre em texto LaTeX seguro: nenhum caractere do
// usuário pode iniciar comandos, abrir grupos, comentar o restante da linha
// ou entrar em modo matemático. Quebras de linha e caracteres de controle
// viram espaço simples.
func Escapar(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 16)
	espacoAnterior := false
	for _, r := range s {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			if !espacoAnterior {
				b.WriteByte(' ')
				espacoAnterior = true
			}
			continue
		}
		espacoAnterior = false
		switch r {
		case '\\':
			b.WriteString(`\textbackslash{}`)
		case '{':
			b.WriteString(`\{`)
		case '}':
			b.WriteString(`\}`)
		case '$':
			b.WriteString(`\$`)
		case '&':
			b.WriteString(`\&`)
		case '#':
			b.WriteString(`\#`)
		case '%':
			b.WriteString(`\%`)
		case '_':
			b.WriteString(`\_`)
		case '^':
			b.WriteString(`\textasciicircum{}`)
		case '~':
			b.WriteString(`\textasciitilde{}`)
		case '[':
			// Evita que "[" seja interpretado como argumento opcional após \\ ou comandos.
			b.WriteString(`{[}`)
		case ']':
			b.WriteString(`{]}`)
		case '\u00A0':
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
