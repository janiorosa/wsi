# Templates dos documentos institucionais

Gerados automaticamente pelo backend com **XeLaTeX** (dentro do Docker). Nenhum usuário compila nada.

| Pasta | Documento | Versão atual |
|---|---|---|
| `ata/ata.tex.tmpl` | ANEXO II – ATA de conclusão de TCC | `ata-v3` |
| `declaracao-antiplagio/declaracao.tex.tmpl` | Declaração anti-plágio | `declaracao-v2` |
| `comum/preambulo.tex.tmpl` | Preâmbulo e cabeçalho institucional (comum aos dois) | — |
| `comum/brasao.png` | Brasão da República (imagem oficial fornecida) | — |

Delimitadores de template: `<< >>` (Go `text/template`), escolhidos para não conflitar com `{ }` do LaTeX.

**Todo valor inserido é escapado** no pacote `internal/latex` antes de chegar ao template
(`%`, `$`, `&`, `#`, `_`, `{`, `}`, `~`, `^`, `\`, `[`, `]`). O template nunca recebe texto cru.

## Variáveis

### Comuns
- `.Cabecalho` — linhas do cabeçalho (`Texto`, `Negrito`), vindas de *Configurações institucionais*.

### ATA (`DadosATA`)
- `.Dia`, `.Mes`, `.Ano`, `.Horario` — **sempre** da sessão/evento atual associado ao trabalho, nunca do relógio.
- `.Local` — local cadastrado no evento/sessão (inserido após “na sala”).
- `.NomeAluno`, `.Matricula`, `.Titulo`.
- `.ListaBanca` — “1 - Examinador 1”, “2 - Examinador 2”, …, e por último o orientador (como no modelo).
- `.NotaFinal` — “87,00” (média simples, 2 casas, meio-para-cima).
- `.Deliberacao` — “Aprovação” (nota final ≥ 60,00) ou “Reprovação”.
- `.Assinaturas` — `Rotulo`/`Nome`: Professor Orientador, Examinador 1..N, Aluno.

### Declaração (`DadosDeclaracao`)
- `.NomeAluno`, `.Matricula`, `.Cidade`, `.Dia`, `.Mes`, `.Ano` (data de emissão do documento).

### Páginas iniciais dos anais (`DadosAnaisAbertura`, `anais/abertura.tex.tmpl`)
Não usa o preâmbulo/cabeçalho institucional. Imagens: `anais/logo-wsi.png` e `anais/logo-ifmg.png` (copiadas para a compilação).
- `.LinhaVolume` — “v.6, n.1, jul. 2026.” (número da edição, mês abreviado e ano do evento).
- `.Instituicao`, `.TituloWorkshop`, `.LocalCapa` — textos da capa (fundo azul).
- `.DataLocal` — “28 de Julho de 2026, Ouro Branco, Minas Gerais”.
- `.TituloFolhaRosto` — “ANAIS DO VI WORKSHOP DE SISTEMAS DE INFORMAÇÃO”.
- `.Organizadores` — apresentadores do evento; `.Campus`, `.Endereco` — rodapé da folha de rosto.
- `.Paragrafos` — editorial já montado em LaTeX a partir de texto escapado (premiados e maior nota em negrito).
- `.Assinatura`, `.CargoAssinatura` — “A e B” / “Coordenadores do VI Workshop de Sistemas de Informação 2026”.

## Ao alterar um template
1. Altere o arquivo e **incremente a versão** em `internal/latex/documentos.go`.
2. Rode os testes de documentos (`docker compose --profile testes run --build --rm testes-backend`).
3. Confira visualmente os PNGs gerados em `dados/testes-visuais/`.
O texto institucional/jurídico não deve ser alterado sem autorização.
