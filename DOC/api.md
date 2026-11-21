# API

- Base: `/api/v1` (o mesmo domínio do frontend, via proxy do nginx).
- Especificação completa: **OpenAPI 3** em `backend/openapi.yaml`, também servida em `GET /api/v1/openapi.yaml`.
- Formato: JSON. Datas de calendário em `AAAA-MM-DD`, horários em `HH:MM`, datas de registro já formatadas em `DD/MM/AAAA HH:MM` (São Paulo).

## Autenticação

1. `POST /auth/entrar {email, senha}` define o cookie `wsi_sessao` (HttpOnly) e devolve a sessão com `csrf`.
2. Toda requisição `POST/PUT/PATCH/DELETE` autenticada envia `X-CSRF-Token: <csrf>`.
3. Usuário ADMIN + DOCENTE: `POST /auth/contexto {contexto: "ADMIN" | "DOCENTE"}`.
4. `GET /auth/sessao` devolve a sessão atual (ou `autenticado: false`).

## Erros

```json
{ "erro": { "codigo": "validacao", "mensagem": "Verifique os campos destacados.", "campos": { "email": "Informe um e-mail válido." } } }
```

| Status | Código | Situação |
|---|---|---|
| 400 | `validacao`, `requisicao_invalida` | Dados inválidos (com `campos`, quando aplicável) |
| 401 | `nao_autenticado`, `credenciais_invalidas`, `primeiro_acesso_pendente` | Sem sessão ou login inválido |
| 403 | `proibido`, `csrf` | Contexto errado, falta de permissão ou CSRF |
| 404 | `nao_encontrado` | Inexistente **ou sem acesso** (prevenção de IDOR) |
| 409 | `conflito`, `aluno_com_orientador`, `senha_ja_definida` | Violação de regra de estado (ex.: fora da vez, ATA já gerada) |
| 413 | `arquivo_grande` | Upload acima do limite |
| 429 | `muitas_tentativas`, `conta_bloqueada_temporariamente` | Limitação de taxa ou bloqueio |

## Paginação, filtros e ordenação

Listagens administrativas aceitam `?pagina=1&tamanho=20` (máximo 100) e devolvem `{itens, total, pagina, tamanho}`. Filtros: `busca`, `perfil`, `estado`. Ordenação: `?ordem=nome` ou `?ordem=-criado` (lista branca de colunas).

## Operações assíncronas

As gerações devolvem `202 {tarefaId}`; o andamento é consultado em `GET /tarefas/{id}` (`pendente | processando | concluida | falhou`, `progresso`). A geração é idempotente: repetir a ação devolve a tarefa ativa já existente.

## Uploads

`multipart/form-data` com o campo `arquivo` (PDF). A assinatura da ATA também envia `posicao` (a posição atual do fluxo), que impede envios duplicados.

## Módulos e rotas

| Módulo | Rotas |
|---|---|
| saúde | `GET /saude` |
| autenticação/conta | `/auth/*`, `/conta` |
| público | `/publico/eventos`, `/publico/eventos/{slug}`, `/publico/publicacoes/{id}/arquivo` |
| admin | `/admin/painel`, `/admin/usuarios`, `/admin/docentes/{id}/aprovar|rejeitar`, `/admin/vinculos`, `/admin/eventos`, `/admin/sessoes`, `/admin/trabalhos`, `/admin/criterios`, `/admin/configuracoes`, `/admin/tarefas`, `/admin/auditoria` |
| docente | `/docente/painel`, `/docente/orientandos`, `/docente/trabalhos/{id}` (evento, banca, critérios, ATA), `/docente/avaliacoes`, `/docente/atas` |
| apresentador | `/apresentador/eventos`, `/apresentador/eventos/{id}/publicacoes`, `/apresentador/publicacoes/{id}/publicar` |
| aluno | `/aluno/painel`, `/aluno/trabalho`, `/aluno/trabalho/arquivos/{tipo}`, `/aluno/orientador`, `/aluno/declaracao/*` |
| arquivos | `/arquivos/{id}`, `/documentos/{id}`, `/declaracoes-assinadas/{id}`, `/atas/{id}`, `/atas/{id}/documento`, `/atas/{id}/assinatura` |

Caminhos físicos nunca são devolvidos: os downloads usam identificadores e passam pela autorização.
