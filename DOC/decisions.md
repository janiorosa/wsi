# Decisões técnicas e pendências

## Decisões

| # | Decisão | Motivo |
|---|---|---|
| 1 | Todo o código, banco, rotas e estados em pt-BR, sem acentos nos identificadores | Pedido do responsável; o sistema nunca terá outro idioma |
| 2 | Monólito Go + fila no PostgreSQL (`FOR UPDATE SKIP LOCKED`), concorrência 1 (máx. 2) | Volume pequeno; sem Redis ou mensageria (especificação) |
| 3 | pgx em **protocolo simples** e SQL explícito | Simplicidade e tipos previsíveis; volume baixo |
| 4 | Migrador próprio, versionado e embutido no binário (`migracoes_aplicadas` + advisory lock) | Sem dependência extra; aplicado automaticamente |
| 5 | XeLaTeX + Latin Modern (fontspec) | A especificação exige XeLaTeX; Latin Modern equivale ao Computer Modern dos modelos |
| 6 | Cabeçalho via `fancyhdr`, com altura medida na compilação | Repetição garantida em todas as páginas, com qualquer número de examinadores |
| 7 | `ulem` (`\textbf{\uline{…}}`) no lugar de `\underline` | Títulos e nomes longos quebram linha sem corte manual |
| 8 | `framed` + blocos de assinatura em `minipage` | A moldura pode continuar na página seguinte sem dividir uma assinatura |
| 9 | qpdf/poppler (CLI) para validar e juntar PDFs | Ferramentas maduras e tolerantes a PDFs reais (assinados, gov.br) |
| 10 | Cada trabalho tem **um** evento atual (`trabalhos.evento_id`) + sessão opcional | Elimina a ambiguidade de “várias associações futuras” |
| 11 | Estado do trabalho **derivado** dos dados (`regras.RecalcularEstadoTrabalho`) | Transições consistentes e controladas pelo backend |
| 12 | Notas em centésimos inteiros; médias com arredondamento meio-para-cima a 2 casas | Mesmo resultado na interface, no banco e na ATA |
| 13 | Critérios do trabalho congelados após a 1ª avaliação finalizada | Fotografia histórica; alterações posteriores não mudam avaliações |
| 14 | ATA gerada **automaticamente** ao concluir a última avaliação (se houver evento) e também por botão | “Automaticamente ou por ação explícita autorizada” |
| 15 | Regeneração do original somente antes da 1ª assinatura; depois, reinício auditado | Impede alterar um documento que já está sendo assinado |
| 16 | Objetos sem acesso respondem 404 | Prevenção de IDOR e de enumeração |
| 17 | Build do frontend com `vite build`; `npm run verificar-tipos` separado | Verificação de tipos no CI/desenvolvimento sem bloquear a imagem |
| 18 | Rótulo **“Professor Orientador”** na assinatura da ATA (o modelo usava “Orientador”) | Seções 9 e 12.2 da especificação |
| 19 | Cabeçalho segundo a seção 14.3: endereço e telefone em linhas separadas, “CEP: 36420-000” e hífen | A especificação (MD) prevalece sobre os PDFs antigos. O texto é configurável pelo ADMIN |
| 20 | Data da declaração = data de emissão | Recomendação operacional da seção 26 |
| 21 | Aprovação com nota final ≥ 60,00; ATA com “Aprovação” ou “Reprovação” | Definição do responsável |
| 22 | Critério geral fixo “Nota” (coluna `fixo`, criado pelo seed, protegido por gatilho e pela API) | Definição do responsável: deve existir sempre e ser avaliado por todos os docentes |
| 23 | Acesso pela LAN em `http://IP:PORTA/` (raiz) com cookie não `Secure` | Implantação em servidor da rede local sem HTTPS |
| 25 | Notas e parecer salvos automaticamente; avaliações pendentes direto no painel do docente | Definição do responsável. A nota fica em rascunho e pode mudar até “Enviar avaliação”; depois, só o orientador reabre (como antes) |
| 26 | Parecer descritivo separado da nota (`avaliacoes.parecer`), visível ao aluno e editável até a geração da ATA | Serve de subsídio para o aluno corrigir o artigo; não entra no cálculo |
| 27 | Premiações por evento com votação dos docentes (até 2 votos por premiação); todo evento nasce com o “Prêmio Destaque: Reconhecendo a Excelência Acadêmica”; empate = todos os mais votados; votação aberta até o evento ficar “Publicado” | Definição do responsável |
| 28 | Anais começam com capa, folha de rosto e editorial gerados em LaTeX (`anais/abertura.tex.tmpl`); organizadores = apresentadores; editorial cita os premiados e o(s) trabalho(s) de maior nota final (sem imprimir a nota); número da edição (`eventos.numero_edicao`) obrigatório | Modelo dos Anais do V WSI (2025). Trechos específicos de uma edição (palestras, datas de vários dias) foram omitidos do texto padrão |
| 29 | Docentes da banca baixam a ATA original e a versão com as assinaturas já coletadas a qualquer momento; o orientador baixa também a declaração gerada | Definição do responsável |
| 30 | Aspas retas (`"`) viram `\textquotedbl{}` no LaTeX | Com o babel em português, `"a`/`"o` virariam ª/º em títulos |
| 24 | Primeiro acesso por nome completo + e-mail + matrícula/SIAPE, sem códigos; “esqueci a senha” = ADMIN redefine o acesso | Definição do responsável. Risco aceito: quem conhecer os três dados de uma conta **ainda sem senha** pode criá-la. Mitigação: só contas sem senha, bloqueio após 5 erros, auditoria. A tabela `tokens_conta` permanece no banco, sem uso |

## Pendências de definição (não inventadas)

1. **SIAPEs ausentes**: docentes sem SIAPE ficam marcados como “dados pendentes de conferência”.
2. **Acesso excepcional do ADMIN a ATAs e declarações**: não implementado, aguardando regra institucional.
3. **Coorientador**: fora do escopo desta versão.
4. **Sumário dos anais** (lista de trabalhos com fotos, como no V WSI): não implementado; a capa, a folha de rosto e o editorial já são gerados.
