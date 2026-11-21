# Arquitetura

## Visão geral

```
Navegador ──HTTP──▶ frontend (nginx: SPA React + proxy /api) ──▶ backend (Go: API REST + trabalhador da fila)
                                                                     │            │
                                                                     ▼            ▼
                                                          PostgreSQL (dados + fila)   volume /dados (PDFs privados)
```

- **Monólito modular**: um único binário Go com API e trabalhador da fila. Não há microsserviços, Redis, Kafka, RabbitMQ nem Kubernetes.
- Volume esperado: um evento com 15 a 20 alunos, no máximo 2 documentos gerados ao mesmo tempo.

## Containers

| Serviço | Imagem | Função | Persistência |
|---|---|---|---|
| `frontend` | nginx 1.27 + build do Vite | SPA, cabeçalhos de segurança, proxy `/api` → backend | — |
| `backend` | Debian 12 + binário Go + TeX Live (XeLaTeX), qpdf e poppler | API, fila, geração e consolidação de PDFs | volume `wsi_arquivos` em `/dados` |
| `postgres` | postgres 16.4 | dados e fila | volume `wsi_postgres_dados` |
| `postgres-testes`, `testes-backend` | perfil `testes` | testes automatizados | tmpfs (descartável) |

O backend roda como usuário não root (`uid 10001`). A construção da imagem compila um documento de verificação: se faltar algum pacote LaTeX, a construção falha.

## Estrutura de pastas

```
backend/
  cmd/servidor/            ponto de entrada (migra, aplica seed, inicia fila e HTTP)
  internal/
    configuracao/          variáveis de ambiente
    banco/                 pool pgx, migrador versionado, migracoes/*.sql
    seguranca/             Argon2id, tokens (somente hash armazenado)
    notas/                 regra de cálculo das notas (aritmética em centésimos)
    datas/                 fuso America/Sao_Paulo e datas por extenso
    latex/                 escape, dados dos documentos, compilação XeLaTeX e validação
    pdf/                   qpdf/poppler: validar, contar páginas, A4, juntar, renderizar
    armazenamento/         arquivos privados com nome aleatório e SHA-256
    fila/                  fila persistente (FOR UPDATE SKIP LOCKED)
    documentos/            processadores: declaração, ATA, apresentações, anais
    regras/                máquina de estados do trabalho
    semente/               carga inicial idempotente
    api/                   HTTP por módulo: autenticacao, admin_usuarios, orientacao, eventos,
                           trabalhos, aluno, docente, avaliacoes, atas, publicacoes, arquivos, admin
  templates/documentos/    ata/, declaracao-antiplagio/, comum/ (preâmbulo, cabeçalho e brasão)
  openapi.yaml
frontend/
  src/api, src/sessao, src/componentes, src/paginas/{publico,admin,docente,aluno}, src/utilitarios
  tests/unidade (Vitest), tests/e2e (Playwright)
DOC/  scripts/  docker-compose.yml  .env.exemplo
```

## Fluxo de documentos

1. A ação do usuário (ou a última avaliação finalizada) cria a tarefa em `tarefas_documentos`, dentro da mesma transação da regra de negócio.
2. O trabalhador (1 ou 2 goroutines) reivindica a tarefa com `UPDATE ... WHERE id = (SELECT ... FOR UPDATE SKIP LOCKED)`.
3. O processador lê os dados atuais do banco, escapa todo o texto, preenche o template (`text/template` com delimitadores `<< >>`) e executa `xelatex -no-shell-escape` em um diretório temporário isolado, com tempo limite.
4. A validação confere: compilação sem erro, nenhum “Missing character”, PDF válido (`qpdf --check`), todas as páginas A4 e a quantidade de páginas esperada. Linhas que ultrapassam a margem ficam registradas como aviso.
5. O PDF é salvo no volume com nome aleatório e SHA-256, o estado é atualizado e o frontend, que consulta `/tarefas/{id}`, libera o download.
6. Se o backend reiniciar, as tarefas em `processando` voltam para `pendente`. Falhas podem ser reprocessadas pelo ADMIN.

## Segurança

- Sessão por cookie `HttpOnly`, `SameSite=Lax` e `Secure` (produção). O token é guardado só como hash, com expiração deslizante de 12 h.
- CSRF: o cabeçalho `X-CSRF-Token` é obrigatório em métodos que alteram estado.
- Autorização sempre no backend: contexto da sessão + vínculo com o objeto (prevenção de IDOR). Objetos sem permissão respondem 404.
- Argon2id (m=64 MiB, t=3, p=2) para senhas. Bloqueio temporário após 5 falhas (login e primeiro acesso). Limitação de taxa por IP nas rotas de autenticação.
- Uploads: limite de tamanho, tipo detectado pelo conteúdo, assinatura `%PDF-` e `qpdf --check`. A extensão é ignorada.
- LaTeX: escape completo de `\ { } $ & # % _ ^ ~ [ ]`, sem shell escape, em diretório temporário isolado e com tempo limite.
- Arquivos privados ficam fora de diretórios públicos, e os downloads sempre passam pelo backend.
- Auditoria das operações sensíveis em `registros_auditoria`.
