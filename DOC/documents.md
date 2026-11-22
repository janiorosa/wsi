# Documentos gerados

## Templates

Ficam em `backend/templates/documentos/` (detalhes das variáveis em `backend/templates/documentos/README.md`):

| Documento | Template | Versão | Referência |
|---|---|---|---|
| ATA de conclusão de TCC (Anexo II) | `ata/ata.tex.tmpl` | `ata-v3` | `ata-e-declaracao/Ata-TCC-Guilherme.tex`, `ATA-TCC-Ludmila.tex` e PDFs |
| Declaração anti-plágio | `declaracao-antiplagio/declaracao.tex.tmpl` | `declaracao-v2` | `ata-e-declaracao/declaração-guilherme.tex` e PDF |
| Preâmbulo e cabeçalho comuns | `comum/preambulo.tex.tmpl` | — | Seção 14.3 da especificação |
| Brasão da República | `comum/brasao.png` | — | `ata-e-declaracao/image.png` |
| Capa, folha de rosto e editorial dos anais | `anais/abertura.tex.tmpl` | `anais-abertura-v1` | Anais do V WSI (2025), 3 primeiras páginas |
| Logotipos dos anais | `anais/logo-wsi.png`, `anais/logo-ifmg.png` | — | Logotipos enviados pelo responsável (fundo xadrez removido) |

O texto institucional e jurídico dos modelos foi copiado **literalmente**.

## Layout

- A4, fonte de 12 pt Latin Modern (equivalente ao Computer Modern dos PDFs de referência, adequado ao XeLaTeX), margens laterais e inferior de 2,5 cm.
- **Cabeçalho em todas as páginas**, começando a 0,5 cm do topo: brasão (largura 6,4 cm, proporção preservada) e as linhas institucionais configuráveis. A altura do cabeçalho é medida na compilação, então o corpo nunca invade o cabeçalho.
- Campos preenchidos em **negrito sublinhado**. Diferente do modelo, que quebrava o título manualmente em 3 linhas, o sublinhado aqui quebra sozinho, então títulos e nomes longos não ficam cortados.
- ATA, página 1: “( ) Publicação…” em branco e “(X) Defesa em sessão pública…” preenchida com horário, local e banca. Como no modelo, os examinadores aparecem primeiro e o orientador por último, seguidos do título.
- ATA, página 2 (`ata-v3`): deliberação (“Aprovação” se a nota final for ≥ 60,00, senão “Reprovação”) com a nota final (“87,00”), uma linha em branco e o título “NOME E ASSINATURA…”. Seguem até **4 blocos de assinatura** (caso usual: Professor Orientador, Examinador 1, Examinador 2, Aluno), cada um com 2,2 cm livres para assinar, linha e identificação abaixo. A moldura é indivisível (como no modelo) e termina com espaço extra após o último bloco. Com mais signatários, os blocos excedentes continuam em nova página, numa nova moldura (até 5 por página). Se qualquer conteúdo ultrapassar a altura da página, a geração falha em vez de cortar texto.
- Declaração (`declaracao-v2`): uma página, com “Ouro Branco, DD de Mês de AAAA” e, abaixo, bloco de assinatura no padrão da ATA (2,8 cm livres, linha e “Aluno: Nome completo” abaixo).

## Fonte dos dados

| Campo | ATA | Declaração |
|---|---|---|
| Data e horário | Sessão do trabalho ou, se não houver, evento atual do trabalho. **Nunca** a data de geração | Data de emissão (geração), fuso de São Paulo |
| Local | Sessão ou evento (“na sala …”) | Cidade das configurações (“Ouro Branco”) |
| Nome e matrícula | Cadastro do aluno | Cadastro do aluno |
| Banca | Congelada na criação da ATA | — |
| Nota | Congelada na criação da ATA | — |

Se a ATA for regenerada antes da primeira assinatura, data, horário e local são relidos do evento.

## Segurança do LaTeX

Todo texto vindo de usuários (nomes, título, local, cabeçalho) passa por `latex.Escapar`: `\ { } $ & # % _ ^ ~ [ ]` são neutralizados e quebras de linha viram espaço. A compilação usa `-no-shell-escape`, roda em diretório temporário próprio e tem tempo limite. Testes garantem que `\input{...}`, `\write18` e similares aparecem como texto.

## Validação de cada PDF gerado

1. O XeLaTeX termina sem erro (`-halt-on-error`).
2. Não há “Missing character”. Um caractere sem glifo na fonte, como um ideograma, faz a geração falhar com mensagem clara, em vez de sumir do documento.
3. `qpdf --check` aprova o arquivo.
4. Todas as páginas são A4 (595,28 × 841,89 pt).
5. A ATA tem pelo menos 2 páginas; a declaração, exatamente 1.
6. Linhas que ultrapassam a margem (“Overfull hbox”) ficam registradas como **aviso** na tarefa.

## Testes visuais

`docker compose --profile testes run --build --rm testes-backend` compila ATAs com 1, 2, 3 e 6 examinadores, uma ATA com textos longos e caracteres especiais, e a declaração. Os PDFs e PNGs ficam em `dados/testes-visuais/` para comparar lado a lado com os PDFs de `ata-e-declaracao/`. Os testes também conferem pelo texto extraído do PDF: cabeçalho em todas as páginas, rótulos, data do evento e caracteres especiais.

## Consolidação (anais e apresentações)

Nos anais, as páginas iniciais (capa, folha de rosto e editorial, compiladas em LaTeX com os dados do evento) vêm antes dos artigos. `qpdf --empty --pages a.pdf b.pdf … -- saida.pdf` preserva tamanho, orientação e qualidade de cada página. Antes da junção, cada arquivo é validado. Depois, o total de páginas é comparado com a soma das partes. O relatório (`itens_publicacao`) lista cada trabalho como incluído (com a versão), ausente ou inválido.

## Alterar um template

1. Edite o `.tex.tmpl` e **incremente a versão** em `backend/internal/latex/documentos.go`.
2. Rode os testes de documentos e confira os PNGs.
3. Não altere o texto institucional ou jurídico sem autorização.
