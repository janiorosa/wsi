package latex

import (
	"strings"
	"testing"
)

func TestEscaparCaracteresEspeciais(t *testing.T) {
	casos := map[string]string{
		`100%`:          `100\%`,
		`R$ 10`:         `R\$ 10`,
		`A & B`:         `A \& B`,
		`#1`:            `\#1`,
		`nome_arquivo`:  `nome\_arquivo`,
		`{x}`:           `\{x\}`,
		`~`:             `\textasciitilde{}`,
		`a^b`:           `a\textasciicircum{}b`,
		`C:\dir`:        `C:\textbackslash{}dir`,
		"linha1\nlinha2": `linha1 linha2`,
		"  espaços   múltiplos ": `espaços múltiplos`,
		`Conclusão, ç ã é`: `Conclusão, ç ã é`,
		`[opcional]`:    `{[}opcional{]}`,
	}
	for entrada, esperado := range casos {
		if obtido := Escapar(entrada); obtido != esperado {
			t.Errorf("Escapar(%q) = %q, esperado %q", entrada, obtido, esperado)
		}
	}
}

func TestEscaparImpedeComandos(t *testing.T) {
	maliciosos := []string{
		`\input{/etc/passwd}`,
		`\write18{rm -rf /}`,
		`\immediate\write18{id}`,
		`$\x$`,
		`\def\a{b}`,
		"%comentário\n\\end{document}",
	}
	for _, m := range maliciosos {
		e := Escapar(m)
		// Após o escape, toda barra invertida deve fazer parte de um comando seguro gerado por nós.
		limpo := strings.NewReplacer(`\textbackslash{}`, "", `\{`, "", `\}`, "", `\$`, "", `\%`, "", `\&`, "", `\#`, "", `\_`, "",
			`\textasciicircum{}`, "", `\textasciitilde{}`, "").Replace(e)
		if strings.ContainsAny(limpo, `\{}$%`) {
			t.Errorf("texto %q gerou saída insegura %q", m, e)
		}
	}
}
