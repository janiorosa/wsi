# Banco de dados

PostgreSQL 16. Estrutura criada pela migração versionada `backend/internal/banco/migracoes/0001_inicial.sql`
(aplicada automaticamente na inicialização e registrada na tabela `migracoes_aplicadas`).

> Este arquivo foi gerado a partir da migração. Ao criar novas migrações (`0002_...sql`), atualize-o.

## Convenções

- Chaves primárias `uuid` (`gen_random_uuid()`), exceto auditoria e histórico (`bigserial`).
- Datas/horas de registro em `timestamptz`; data e horário de apresentação em `date`/`time` locais de `America/Sao_Paulo`.
- Notas em `numeric(5,2)` com `CHECK` de 0 a 100. **Não existe coluna de peso.**
- Situações mudam por estado (`situacao`, `estado`) e histórico, sem apagar registros.
- Caminhos físicos de arquivos nunca são expostos: o banco guarda uma chave relativa ao volume `/dados/arquivos`.

## Regras de integridade principais

| Regra | Implementação |
|---|---|
| ALUNO não acumula perfis | gatilho `perfis_usuario_exclusividade_aluno` |
| Um orientador ativo por aluno | índice único parcial `vinculos_orientacao_ativo_unico` |
| Um trabalho em andamento por aluno | índice único parcial `trabalhos_aluno_ativo_unico` |
| Docente não se repete na banca | índice único parcial `membros_banca_docente_unico` |
| Uma ATA ativa por trabalho | índice único parcial `atas_ativa_unica` |
| Ordem de assinatura única por ATA | `UNIQUE (ata_id, ordem)` e `UNIQUE (ata_id, signatario_id)` |
| Sem tarefa duplicada (duplo clique) | índice único parcial `tarefas_documentos_ativa_unica` |
| Uma publicação publicada por tipo e evento | índice único parcial `publicacoes_evento_publicada_unica` |
| Escala de 0 a 100 | `CHECK` em critérios, notas e notas calculadas |
| Critério “Nota” sempre presente e ativo | coluna `fixo` + gatilho `criterios_avaliacao_fixo_sem_exclusao` (migração 0002) + seed |

## Tabelas

### `usuarios`

Contas de acesso (todas as pessoas). Senha somente como hash Argon2id.

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `nome_completo` | text NOT NULL CHECK (length(trim(nome_completo)) > 0) |
| `email` | text NOT NULL CHECK (email = lower(email)) |
| `telefone` | text NOT NULL DEFAULT '' |
| `identificador_institucional` | text NOT NULL DEFAULT '' |
| `tipo_identificador` | text NOT NULL DEFAULT '' CHECK (tipo_identificador IN ('', 'SIAPE', 'MATRICULA')) |
| `hash_senha` | text |
| `situacao` | text NOT NULL DEFAULT 'ativo' CHECK (situacao IN ('ativo', 'aguardando_primeiro_acesso', 'bloqueado')) |
| `tentativas_falhas` | integer NOT NULL DEFAULT 0 |
| `bloqueado_ate` | timestamptz |
| `dados_pendentes_conferencia` | boolean NOT NULL DEFAULT false |
| `criado_em` | timestamptz NOT NULL DEFAULT now() |
| `atualizado_em` | timestamptz NOT NULL DEFAULT now() |

Índices:
- `usuarios_email_unico` (único): `(email)`
- `usuarios_nome_idx`: `(lower(nome_completo))`

### `perfis_usuario`

Perfis globais: ADMIN, DOCENTE, ALUNO. Gatilho garante que ALUNO é exclusivo.

| Coluna | Tipo e restrições |
|---|---|
| `usuario_id` | uuid NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE |
| `perfil` | text NOT NULL CHECK (perfil IN ('ADMIN', 'DOCENTE', 'ALUNO')) |
| `concedido_por` | uuid REFERENCES usuarios(id) |
| `concedido_em` | timestamptz NOT NULL DEFAULT now() |

Restrições da tabela: `PRIMARY KEY (usuario_id, perfil)`

### `perfis_docente`

Solicitação/aprovação do perfil DOCENTE e SIAPE. O perfil DOCENTE só entra em perfis_usuario após aprovação.

| Coluna | Tipo e restrições |
|---|---|
| `usuario_id` | uuid PRIMARY KEY REFERENCES usuarios(id) ON DELETE CASCADE |
| `siape` | text NOT NULL DEFAULT '' |
| `situacao_aprovacao` | text NOT NULL DEFAULT 'pendente' CHECK (situacao_aprovacao IN ('pendente', 'aprovado', 'rejeitado')) |
| `aprovado_por` | uuid REFERENCES usuarios(id) |
| `aprovado_em` | timestamptz |
| `motivo_rejeicao` | text NOT NULL DEFAULT '' |

Índices:
- `perfis_docente_situacao_idx`: `(situacao_aprovacao)`

### `perfis_aluno`

Matrícula (única quando preenchida) e curso do aluno.

| Coluna | Tipo e restrições |
|---|---|
| `usuario_id` | uuid PRIMARY KEY REFERENCES usuarios(id) ON DELETE CASCADE |
| `matricula` | text NOT NULL DEFAULT '' |
| `curso` | text NOT NULL DEFAULT 'Sistemas de Informação' |

Índices:
- `perfis_aluno_matricula_unica` (único): `(matricula)` onde `matricula <> ''`

### `tokens_conta`

Reservada para códigos temporários. **Sem uso nesta versão**: o primeiro acesso é feito por nome, e-mail e matrícula/SIAPE.

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `usuario_id` | uuid NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE |
| `hash_token` | text NOT NULL UNIQUE |
| `tipo` | text NOT NULL CHECK (tipo IN ('instalacao', 'primeiro_acesso', 'redefinicao_senha')) |
| `expira_em` | timestamptz NOT NULL |
| `usado_em` | timestamptz |
| `criado_por` | uuid REFERENCES usuarios(id) |
| `criado_em` | timestamptz NOT NULL DEFAULT now() |

Índices:
- `tokens_conta_usuario_idx`: `(usuario_id)`

### `sessoes`

Sessões de login (cookie HttpOnly). Guarda o contexto escolhido (ADMIN/DOCENTE/ALUNO) e o token CSRF.

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `hash_token` | text NOT NULL UNIQUE |
| `usuario_id` | uuid NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE |
| `contexto` | text CHECK (contexto IN ('ADMIN', 'DOCENTE', 'ALUNO')) |
| `token_csrf` | text NOT NULL |
| `criada_em` | timestamptz NOT NULL DEFAULT now() |
| `expira_em` | timestamptz NOT NULL |
| `ultimo_uso_em` | timestamptz NOT NULL DEFAULT now() |

Índices:
- `sessoes_usuario_idx`: `(usuario_id)`

### `registros_auditoria`

Auditoria de operações sensíveis (sem senhas ou segredos).

| Coluna | Tipo e restrições |
|---|---|
| `id` | bigserial PRIMARY KEY |
| `ator_id` | uuid REFERENCES usuarios(id) |
| `acao` | text NOT NULL |
| `entidade` | text NOT NULL |
| `entidade_id` | text NOT NULL DEFAULT '' |
| `dados` | jsonb NOT NULL DEFAULT '{}'::jsonb |
| `ip` | text NOT NULL DEFAULT '' |
| `criado_em` | timestamptz NOT NULL DEFAULT now() |

Índices:
- `registros_auditoria_criado_idx`: `(criado_em DESC)`
- `registros_auditoria_entidade_idx`: `(entidade, entidade_id)`

### `configuracoes_institucionais`

Cabeçalho institucional dos documentos (JSON) e cidade.

| Coluna | Tipo e restrições |
|---|---|
| `chave` | text PRIMARY KEY |
| `valor` | text NOT NULL |
| `atualizado_por` | uuid REFERENCES usuarios(id) |
| `atualizado_em` | timestamptz NOT NULL DEFAULT now() |

### `vinculos_orientacao`

Histórico de orientação. Índice único parcial: um vínculo ativo por aluno.

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `aluno_id` | uuid NOT NULL REFERENCES usuarios(id) |
| `docente_id` | uuid NOT NULL REFERENCES usuarios(id) |
| `criado_por` | uuid NOT NULL REFERENCES usuarios(id) |
| `inicio` | timestamptz NOT NULL DEFAULT now() |
| `fim` | timestamptz |
| `situacao` | text NOT NULL DEFAULT 'ativo' CHECK (situacao IN ('ativo', 'encerrado')) |
| `encerrado_por` | uuid REFERENCES usuarios(id) |
| `motivo_encerramento` | text NOT NULL DEFAULT '' |

Restrições da tabela: `CHECK (aluno_id <> docente_id)`

Índices:
- `vinculos_orientacao_ativo_unico` (único): `(aluno_id)` onde `situacao = 'ativo'`
- `vinculos_orientacao_docente_idx`: `(docente_id, situacao)`

### `eventos`

Eventos. Data, horário e local são a fonte oficial da ATA.

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `nome` | text NOT NULL CHECK (length(trim(nome)) > 0) |
| `edicao` | text NOT NULL DEFAULT '' |
| `descricao` | text NOT NULL DEFAULT '' |
| `data_apresentacao` | date NOT NULL |
| `horario_apresentacao` | time NOT NULL |
| `local` | text NOT NULL CHECK (length(trim(local)) > 0) |
| `campus` | text NOT NULL DEFAULT 'Ouro Branco' |
| `estado` | text NOT NULL DEFAULT 'rascunho' CHECK (estado IN ('rascunho', 'aberto', 'em_preparacao', 'em_andamento', 'encerrado', 'publicado')) |
| `prazo_submissao` | date |
| `prazo_apresentacao` | date |
| `informacoes_publicas` | text NOT NULL DEFAULT '' |
| `slug` | text NOT NULL UNIQUE |
| `capa_chave` | text |
| `capa_mime` | text |
| `criado_por` | uuid NOT NULL REFERENCES usuarios(id) |
| `criado_em` | timestamptz NOT NULL DEFAULT now() |
| `atualizado_em` | timestamptz NOT NULL DEFAULT now() |

Índices:
- `eventos_data_idx`: `(data_apresentacao)`
- `eventos_estado_idx`: `(estado)`

### `sessoes_evento`

Sessões opcionais do evento (várias datas/horários/locais).

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `evento_id` | uuid NOT NULL REFERENCES eventos(id) ON DELETE CASCADE |
| `titulo` | text NOT NULL |
| `data` | date NOT NULL |
| `horario` | time NOT NULL |
| `local` | text NOT NULL CHECK (length(trim(local)) > 0) |
| `ordem` | integer NOT NULL DEFAULT 0 |

Índices:
- `sessoes_evento_evento_idx`: `(evento_id)`

### `apresentadores_evento`

Atribuição de APRESENTADOR (docente) por evento, com permissão de publicação.

| Coluna | Tipo e restrições |
|---|---|
| `evento_id` | uuid NOT NULL REFERENCES eventos(id) ON DELETE CASCADE |
| `docente_id` | uuid NOT NULL REFERENCES usuarios(id) |
| `pode_publicar` | boolean NOT NULL DEFAULT true |
| `atribuido_por` | uuid NOT NULL REFERENCES usuarios(id) |
| `atribuido_em` | timestamptz NOT NULL DEFAULT now() |

Restrições da tabela: `PRIMARY KEY (evento_id, docente_id)`

Índices:
- `apresentadores_evento_docente_idx`: `(docente_id)`

### `trabalhos`

TCCs. Um trabalho em andamento por aluno. evento_id é o evento atual (único) do trabalho.

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `aluno_id` | uuid NOT NULL REFERENCES usuarios(id) |
| `orientador_id` | uuid REFERENCES usuarios(id) |
| `evento_id` | uuid REFERENCES eventos(id) |
| `sessao_id` | uuid REFERENCES sessoes_evento(id) ON DELETE SET NULL |
| `ordem_programacao` | integer |
| `titulo` | text NOT NULL DEFAULT '' |
| `resumo` | text NOT NULL DEFAULT '' |
| `link_overleaf` | text NOT NULL DEFAULT '' |
| `estado` | text NOT NULL DEFAULT 'rascunho' CHECK (estado IN ('rascunho', 'submetido', 'aguardando_avaliadores', 'aguardando_avaliacoes', 'avaliacoes_concluidas', 'ata_aguardando_geracao', 'ata_gerada', 'assinaturas_em_andamento', 'ata_finalizada', 'documentacao_pendente', 'concluido', 'cancelado')) |
| `submetido_em` | timestamptz |
| `excecao_evento_passado` | text NOT NULL DEFAULT '' |
| `criado_em` | timestamptz NOT NULL DEFAULT now() |
| `atualizado_em` | timestamptz NOT NULL DEFAULT now() |

Índices:
- `trabalhos_aluno_ativo_unico` (único): `(aluno_id)` onde `estado NOT IN ('cancelado', 'concluido')`
- `trabalhos_evento_idx`: `(evento_id, ordem_programacao)`
- `trabalhos_orientador_idx`: `(orientador_id)`
- `trabalhos_estado_idx`: `(estado)`

### `arquivos_trabalho`

Versões de artigo e apresentação (nunca sobrescritas).

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `trabalho_id` | uuid NOT NULL REFERENCES trabalhos(id) |
| `tipo` | text NOT NULL CHECK (tipo IN ('artigo', 'apresentacao')) |
| `nome_original` | text NOT NULL |
| `chave_armazenamento` | text NOT NULL UNIQUE |
| `mime` | text NOT NULL |
| `tamanho` | bigint NOT NULL |
| `sha256` | text NOT NULL |
| `versao` | integer NOT NULL |
| `enviado_por` | uuid NOT NULL REFERENCES usuarios(id) |
| `enviado_em` | timestamptz NOT NULL DEFAULT now() |

Restrições da tabela: `UNIQUE (trabalho_id, tipo, versao)`

### `historico_estados_trabalho`

Transições de estado do trabalho.

| Coluna | Tipo e restrições |
|---|---|
| `id` | bigserial PRIMARY KEY |
| `trabalho_id` | uuid NOT NULL REFERENCES trabalhos(id) |
| `estado_anterior` | text NOT NULL |
| `estado_novo` | text NOT NULL |
| `ator_id` | uuid REFERENCES usuarios(id) |
| `observacao` | text NOT NULL DEFAULT '' |
| `criado_em` | timestamptz NOT NULL DEFAULT now() |

Índices:
- `historico_estados_trabalho_idx`: `(trabalho_id, criado_em)`

### `membros_banca`

Banca: orientador (ordem 0) + examinadores (ordem 1..n). Quantidade variável.

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `trabalho_id` | uuid NOT NULL REFERENCES trabalhos(id) |
| `docente_id` | uuid NOT NULL REFERENCES usuarios(id) |
| `papel` | text NOT NULL CHECK (papel IN ('orientador', 'examinador')) |
| `ordem` | integer NOT NULL |
| `obrigatorio` | boolean NOT NULL DEFAULT true |
| `situacao` | text NOT NULL DEFAULT 'ativo' CHECK (situacao IN ('ativo', 'removido')) |
| `designado_por` | uuid NOT NULL REFERENCES usuarios(id) |
| `designado_em` | timestamptz NOT NULL DEFAULT now() |

Índices:
- `membros_banca_docente_unico` (único): `(trabalho_id, docente_id)` onde `situacao = 'ativo'`
- `membros_banca_docente_idx`: `(docente_id, situacao)`

### `criterios_avaliacao`

Critérios gerais (0 a 100, sem peso).

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `nome` | text NOT NULL CHECK (length(trim(nome)) > 0) |
| `descricao` | text NOT NULL DEFAULT '' |
| `nota_minima` | numeric(5,2) NOT NULL DEFAULT 0 CHECK (nota_minima = 0) |
| `nota_maxima` | numeric(5,2) NOT NULL DEFAULT 100 CHECK (nota_maxima = 100) |
| `ativo` | boolean NOT NULL DEFAULT true |
| `ordem` | integer NOT NULL DEFAULT 0 |
| `criado_em` | timestamptz NOT NULL DEFAULT now() |
| `fixo` | boolean NOT NULL DEFAULT false — migração 0002; `CHECK (NOT fixo OR ativo)`, índice único parcial `criterios_avaliacao_fixo_unico` e gatilho que impede excluir o critério fixo “Nota” |

### `criterios_trabalho`

Fotografia dos critérios aplicados a cada trabalho (gerais + extras). Congelada após a 1ª avaliação finalizada.

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `trabalho_id` | uuid NOT NULL REFERENCES trabalhos(id) |
| `criterio_geral_id` | uuid REFERENCES criterios_avaliacao(id) |
| `origem` | text NOT NULL CHECK (origem IN ('geral', 'extra')) |
| `nome` | text NOT NULL |
| `descricao` | text NOT NULL DEFAULT '' |
| `nota_minima` | numeric(5,2) NOT NULL DEFAULT 0 CHECK (nota_minima = 0) |
| `nota_maxima` | numeric(5,2) NOT NULL DEFAULT 100 CHECK (nota_maxima = 100) |
| `ordem` | integer NOT NULL |
| `criado_por` | uuid REFERENCES usuarios(id) |
| `criado_em` | timestamptz NOT NULL DEFAULT now() |

Índices:
- `criterios_trabalho_trabalho_idx`: `(trabalho_id, ordem)`
- `criterios_trabalho_geral_unico` (único): `(trabalho_id, criterio_geral_id)` onde `criterio_geral_id IS NOT NULL`

### `avaliacoes`

Avaliação de cada membro da banca; nota calculada e versão da regra.

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `trabalho_id` | uuid NOT NULL REFERENCES trabalhos(id) |
| `avaliador_id` | uuid NOT NULL REFERENCES usuarios(id) |
| `estado` | text NOT NULL DEFAULT 'rascunho' CHECK (estado IN ('rascunho', 'finalizada')) |
| `nota_calculada` | numeric(5,2) CHECK (nota_calculada BETWEEN 0 AND 100) |
| `enviada_em` | timestamptz |
| `versao_regra` | text NOT NULL DEFAULT '' |
| `criado_em` | timestamptz NOT NULL DEFAULT now() |
| `atualizado_em` | timestamptz NOT NULL DEFAULT now() |

Restrições da tabela: `UNIQUE (trabalho_id, avaliador_id)`

### `notas_avaliacao`

Nota (0 a 100) por critério e comentário.

| Coluna | Tipo e restrições |
|---|---|
| `avaliacao_id` | uuid NOT NULL REFERENCES avaliacoes(id) ON DELETE CASCADE |
| `criterio_trabalho_id` | uuid NOT NULL REFERENCES criterios_trabalho(id) |
| `nota` | numeric(5,2) NOT NULL CHECK (nota BETWEEN 0 AND 100) |
| `comentario` | text NOT NULL DEFAULT '' |

Restrições da tabela: `PRIMARY KEY (avaliacao_id, criterio_trabalho_id)`

### `atas`

ATA de cada trabalho: nota final congelada, original LaTeX e SOMENTE a última versão assinada.

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `trabalho_id` | uuid NOT NULL REFERENCES trabalhos(id) |
| `versao` | integer NOT NULL |
| `estado` | text NOT NULL CHECK (estado IN ('gerando', 'aguardando_assinaturas', 'finalizada', 'cancelada')) |
| `nota_final` | numeric(5,2) NOT NULL CHECK (nota_final BETWEEN 0 AND 100) |
| `versao_regra` | text NOT NULL |
| `versao_template` | text NOT NULL DEFAULT '' |
| `chave_original` | text |
| `sha256_original` | text |
| `chave_assinado` | text |
| `sha256_assinado` | text |
| `posicao_atual` | integer NOT NULL DEFAULT 1 |
| `erro_geracao` | text NOT NULL DEFAULT '' |
| `gerado_por` | uuid NOT NULL REFERENCES usuarios(id) |
| `gerado_em` | timestamptz |
| `criado_em` | timestamptz NOT NULL DEFAULT now() |
| `finalizada_em` | timestamptz |
| `motivo_cancelamento` | text NOT NULL DEFAULT '' |

Restrições da tabela: `UNIQUE (trabalho_id, versao)`

Índices:
- `atas_ativa_unica` (único): `(trabalho_id)` onde `estado <> 'cancelada'`

### `assinaturas_ata`

Ordem congelada de signatários da ATA (1 aluno, 2 orientador, 3.. examinadores) e metadados de cada envio.

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `ata_id` | uuid NOT NULL REFERENCES atas(id) |
| `signatario_id` | uuid NOT NULL REFERENCES usuarios(id) |
| `tipo` | text NOT NULL CHECK (tipo IN ('aluno', 'orientador', 'examinador')) |
| `rotulo` | text NOT NULL |
| `ordem` | integer NOT NULL |
| `estado` | text NOT NULL DEFAULT 'pendente' CHECK (estado IN ('pendente', 'assinada')) |
| `enviado_em` | timestamptz |
| `sha256` | text |
| `observacao` | text NOT NULL DEFAULT '' |

Restrições da tabela: `UNIQUE (ata_id, ordem)`; `UNIQUE (ata_id, signatario_id)`

Índices:
- `assinaturas_ata_signatario_idx`: `(signatario_id, estado)`

### `documentos_gerados`

Declarações anti-plágio geradas (versões).

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `tipo` | text NOT NULL CHECK (tipo IN ('declaracao_antiplagio')) |
| `entidade_id` | uuid NOT NULL |
| `versao` | integer NOT NULL |
| `chave_armazenamento` | text NOT NULL UNIQUE |
| `sha256` | text NOT NULL |
| `tamanho` | bigint NOT NULL |
| `versao_template` | text NOT NULL |
| `gerado_por` | uuid NOT NULL REFERENCES usuarios(id) |
| `gerado_em` | timestamptz NOT NULL DEFAULT now() |

Restrições da tabela: `UNIQUE (tipo, entidade_id, versao)`

### `declaracoes_assinadas`

Declarações anti-plágio assinadas enviadas pelo aluno (versões).

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `aluno_id` | uuid NOT NULL REFERENCES usuarios(id) |
| `nome_original` | text NOT NULL |
| `chave_armazenamento` | text NOT NULL UNIQUE |
| `tamanho` | bigint NOT NULL |
| `sha256` | text NOT NULL |
| `versao` | integer NOT NULL |
| `enviado_por` | uuid NOT NULL REFERENCES usuarios(id) |
| `enviado_em` | timestamptz NOT NULL DEFAULT now() |

Restrições da tabela: `UNIQUE (aluno_id, versao)`

### `tarefas_documentos`

Fila assíncrona de documentos (PostgreSQL). Índice único parcial evita tarefas duplicadas.

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `tipo` | text NOT NULL CHECK (tipo IN ('gerar_declaracao', 'gerar_ata', 'gerar_apresentacoes', 'gerar_anais')) |
| `entidade_id` | uuid NOT NULL |
| `parametros` | jsonb NOT NULL DEFAULT '{}'::jsonb |
| `estado` | text NOT NULL DEFAULT 'pendente' CHECK (estado IN ('pendente', 'processando', 'concluida', 'falhou')) |
| `progresso` | integer NOT NULL DEFAULT 0 CHECK (progresso BETWEEN 0 AND 100) |
| `tentativas` | integer NOT NULL DEFAULT 0 |
| `erro` | text NOT NULL DEFAULT '' |
| `avisos` | text NOT NULL DEFAULT '' |
| `resultado_id` | uuid |
| `solicitado_por` | uuid NOT NULL REFERENCES usuarios(id) |
| `criado_em` | timestamptz NOT NULL DEFAULT now() |
| `iniciado_em` | timestamptz |
| `concluido_em` | timestamptz |

Índices:
- `tarefas_documentos_ativa_unica` (único): `(tipo, entidade_id)
    WHERE estado IN ('pendente', 'processando')`
- `tarefas_documentos_estado_idx`: `(estado, criado_em)`

### `publicacoes_evento`

Anais e apresentações consolidadas (versões, revisão e publicação).

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `evento_id` | uuid NOT NULL REFERENCES eventos(id) |
| `tipo` | text NOT NULL CHECK (tipo IN ('anais', 'apresentacoes')) |
| `versao` | integer NOT NULL |
| `chave_armazenamento` | text |
| `sha256` | text |
| `tamanho` | bigint |
| `total_paginas` | integer |
| `estado` | text NOT NULL CHECK (estado IN ('em_preparacao', 'aguardando_revisao', 'publicado', 'substituido', 'falhou')) |
| `erro` | text NOT NULL DEFAULT '' |
| `gerado_por` | uuid NOT NULL REFERENCES usuarios(id) |
| `gerado_em` | timestamptz |
| `publicado_por` | uuid REFERENCES usuarios(id) |
| `publicado_em` | timestamptz |
| `criado_em` | timestamptz NOT NULL DEFAULT now() |

Restrições da tabela: `UNIQUE (evento_id, tipo, versao)`

Índices:
- `publicacoes_evento_publicada_unica` (único): `(evento_id, tipo)` onde `estado = 'publicado'`

### `itens_publicacao`

Relatório de inclusão de cada trabalho na publicação (incluído/ausente/inválido).

| Coluna | Tipo e restrições |
|---|---|
| `id` | uuid PRIMARY KEY DEFAULT gen_random_uuid() |
| `publicacao_id` | uuid NOT NULL REFERENCES publicacoes_evento(id) ON DELETE CASCADE |
| `trabalho_id` | uuid NOT NULL REFERENCES trabalhos(id) |
| `arquivo_id` | uuid REFERENCES arquivos_trabalho(id) |
| `posicao` | integer NOT NULL |
| `resultado` | text NOT NULL CHECK (resultado IN ('incluido', 'ausente', 'invalido')) |
| `detalhe` | text NOT NULL DEFAULT '' |
| `paginas` | integer |

Índices:
- `itens_publicacao_idx`: `(publicacao_id, posicao)`

## Relacionamentos

```
usuarios 1─N perfis_usuario            usuarios 1─1 perfis_docente / perfis_aluno
usuarios(aluno) 1─N vinculos_orientacao N─1 usuarios(docente)
eventos 1─N sessoes_evento             eventos 1─N apresentadores_evento N─1 usuarios
usuarios(aluno) 1─N trabalhos N─1 eventos (evento atual) / sessoes_evento
trabalhos 1─N arquivos_trabalho | historico_estados_trabalho | membros_banca | criterios_trabalho | avaliacoes
avaliacoes 1─N notas_avaliacao N─1 criterios_trabalho
trabalhos 1─N atas (no máximo 1 ativa) 1─N assinaturas_ata
usuarios(aluno) 1─N documentos_gerados (declaração) | declaracoes_assinadas
eventos 1─N publicacoes_evento 1─N itens_publicacao N─1 trabalhos
```
