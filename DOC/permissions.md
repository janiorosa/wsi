# Permissões

Todas as verificações acontecem no **backend**. Esconder botões no frontend é só conveniência. Objetos sem permissão respondem **404**, para não revelar a existência por UUID.

## Contexto da sessão

| Perfis do usuário | Ao entrar |
|---|---|
| ALUNO | Contexto ALUNO |
| DOCENTE | Contexto DOCENTE |
| ADMIN | Contexto ADMIN |
| ADMIN + DOCENTE | Tela “ENTRAR COMO ADMIN / ENTRAR COMO DOCENTE”; troca possível depois |
| Docente pendente ou rejeitado | Somente a página “aguardando aprovação” |

Rotas `/admin/*` exigem o contexto ADMIN; `/docente/*` e `/apresentador/*`, o contexto DOCENTE; `/aluno/*`, o contexto ALUNO. Os perfis são relidos do banco a cada requisição, então uma remoção vale imediatamente.

## Matriz por objeto

| Objeto | ALUNO (autor) | Orientador | Examinador da banca | Apresentador do evento | ADMIN | Público |
|---|---|---|---|---|---|---|
| Título, resumo, artigo e apresentação | ✅ | ✅ | ✅ | ✅ (arquivos e título) | Lista de trabalhos (sem arquivos) | Título na programação |
| Notas individuais | — | ✅ (todas) | Somente a própria | — | — | — |
| ATA (durante o fluxo) | Somente na sua vez | Somente na sua vez | Somente na sua vez | — | — | — |
| ATA final | ✅ | ✅ | ✅ | — | — | — |
| Declaração gerada | ✅ | — | — | — | — | — |
| Declaração assinada | ✅ | ✅ (orientador ativo) | — | — | — | — |
| Anais/apresentações (rascunho) | — | — | — | ✅ conferir/publicar | Situação | — |
| Anais/apresentações publicados | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

O ADMIN **não** recebe acesso automático às ATAs nem às declarações. Um eventual acesso excepcional depende de regra institucional ainda não definida.

## Ações por perfil

- **ADMIN**: eventos, sessões, apresentadores, programação, usuários, aprovação de docentes, redefinição de acesso (exceto da própria conta), perfis ADMIN/DOCENTE (nunca em conta de aluno; o último ADMIN não pode ser removido), vínculos, critérios gerais, dados institucionais, fila de documentos, auditoria e exceção de evento passado.
- **DOCENTE (orientador do trabalho)**: associar evento futuro, definir banca, critérios extras, reabrir avaliação, gerar, regenerar ou reiniciar a ATA e baixar a declaração assinada.
- **DOCENTE (membro da banca)**: avaliar e assinar a ATA na sua vez.
- **APRESENTADOR**: ordenar a programação, gerar apresentações e anais, conferir e publicar (se tiver a permissão `pode_publicar`).
- **ALUNO**: o próprio trabalho, arquivos, orientador, declaração e assinatura da própria ATA.
