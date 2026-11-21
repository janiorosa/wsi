# Getting Started — como executar

## Requisitos

- Docker Engine 24+ e Docker Compose v2 (`docker compose`).
- Cerca de 3 GB livres para as imagens: o TeX Live ocupa a maior parte.
- **Não** é preciso instalar Go, Node, LaTeX ou PostgreSQL no servidor. Tudo roda nos containers.

## Versões fixadas

| Componente | Versão |
|---|---|
| Go (compilação) | 1.23.4 (`golang:1.23.4-bookworm`) |
| Imagem de execução do backend | `debian:bookworm-slim` + TeX Live do Debian 12 (XeLaTeX), qpdf, poppler-utils |
| PostgreSQL | 16.4 (`postgres:16.4-alpine`) |
| Node (compilação do frontend) | 22.11.0 |
| nginx | 1.27.2 |
| Dependências Go | `backend/go.mod` (chi 5.1.0, pgx 5.7.1, x/crypto 0.29.0, x/text 0.20.0) |
| Dependências do frontend | `frontend/package.json` (versões exatas) |

**Atualizar versões:** altere `go.mod`, `package.json` ou as tags das imagens, reconstrua (`docker compose build`), rode os testes e confira os PDFs de teste visual.

## Primeira instalação

```bash
cp .env.exemplo .env
# edite .env: POSTGRES_PASSWORD (só letras e números), WSI_PORTA, WSI_COOKIE_SEGURO
docker compose up -d --build
```

Acesse `http://IP-DO-SERVIDOR:8080` de qualquer computador da rede (porta definida em `WSI_PORTA`).

### Acesso pela rede local (LAN)

- O frontend publica a porta em todas as interfaces do servidor; basta abrir `http://IP-DO-SERVIDOR:8080` em outro computador. Descubra o IP com `hostname -I` (Linux).
- Por HTTP, mantenha `WSI_COOKIE_SEGURO=false`. Se estiver `true` sem HTTPS, o navegador descarta o cookie de sessão e o login volta sempre para a tela de entrada.
- Se a porta 8080 já estiver em uso, altere `WSI_PORTA` no `.env` e rode `docker compose up -d`.
- Se não abrir de outro computador, libere a porta no firewall do servidor (ex.: `sudo ufw allow 8080/tcp`).

### Migrations e seed

Rodam **automaticamente** a cada inicialização do backend:

- as migrações versionadas (`backend/internal/banco/migracoes/NNNN_*.sql`) são aplicadas uma única vez cada uma;
- o seed é idempotente e nunca sobrescreve dados alterados depois. Ele cadastra os docentes informados, o ADMIN inicial e os dados institucionais. **Não** cadastra critério de avaliação: o ADMIN cadastra o critério geral confirmado pela instituição.

### Primeiro acesso (inclusive do ADMIN)

Ninguém recebe código ou senha provisória. Quem ainda não tem senha abre **`/primeiro-acesso`** (link “Primeiro acesso / esqueci a senha” na tela de login) e informa:

- **nome completo**, como cadastrado (maiúsculas, acentos e espaços extras são ignorados);
- **e-mail**;
- **matrícula** (aluno) ou **SIAPE** (docente). Pontos, traços e zeros à esquerda são ignorados.

Se tudo conferir e a conta ainda não tiver senha, a pessoa cria a senha. O ADMIN inicial é `WSI_EMAIL_ADMIN_INICIAL` (Janio Rosa da Silva, SIAPE 1752938). Docentes do seed sem SIAPE informado precisam que o ADMIN complete o SIAPE em *Usuários → Editar* antes do primeiro acesso.

**Esqueceu a senha?** O ADMIN usa *Usuários → Redefinir acesso* (ícone de cadeado). A senha é apagada, as sessões são encerradas e a pessoa refaz o primeiro acesso. Depois que a senha existe, a tela de primeiro acesso recusa novas tentativas. Cinco tentativas com dados errados bloqueiam a conta por 15 minutos.

## Variáveis de ambiente (`.env`)

| Variável | Padrão | Descrição |
|---|---|---|
| `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD` | wsi / wsi / — | Banco de dados |
| `WSI_PORTA` | 8080 | Porta publicada pelo frontend |
| `WSI_COOKIE_SEGURO` | false | `false` para acesso por HTTP (LAN). `true` somente com HTTPS |
| `WSI_EMAIL_ADMIN_INICIAL` | janio.silva@ifmg.edu.br | Quem recebe o perfil ADMIN inicial |
| `WSI_CONCORRENCIA_FILA` | 1 | Documentos gerados ao mesmo tempo (1 ou 2) |
| `WSI_TAMANHO_MAXIMO_UPLOAD_MB` | 30 | Limite dos PDFs enviados |
| `WSI_ORIGENS_PERMITIDAS` | vazio | Origens CORS extras (apenas para desenvolvimento) |

## Execução local para desenvolvimento

```bash
# banco
docker compose up -d postgres
# backend (precisa de Go 1.23 e, para gerar PDFs, xelatex + qpdf + poppler instalados localmente)
cd backend
WSI_URL_BANCO=postgres://wsi:SENHA@localhost:5432/wsi?sslmode=disable \
WSI_DIRETORIO_DADOS=./dados-dev WSI_DIRETORIO_TEMPLATES=./templates/documentos \
WSI_COOKIE_SEGURO=false go run ./cmd/servidor
# frontend (Node 22)
cd frontend && npm install && npm run dev   # http://localhost:5173 (proxy /api → :8080)
```

Para acessar o banco pela máquina local, publique a porta 5432 com um `docker-compose.override.yml`.

## Testes

```bash
# Backend: unidade + integração (PostgreSQL de teste descartável) + compilação real dos PDFs
docker compose --profile testes run --build --rm testes-backend
# PDFs e PNGs para conferência visual ficam em ./dados/testes-visuais/

# Frontend: unidade
cd frontend && npm install && npm test

# Ponta a ponta (com o sistema rodando)
cd frontend && npx playwright install chromium && WSI_URL_BASE=http://localhost:8080 npm run test:e2e
```

## Geração automática de documentos

Ninguém executa `xelatex` manualmente. Ao clicar em “GERAR …”, o backend registra uma tarefa em `tarefas_documentos`. O trabalhador interno compila o template no container, valida o PDF (A4, páginas e caracteres) e o armazena no volume `wsi_arquivos`. A tela acompanha o progresso até liberar o download. Veja [documents.md](documents.md).

## Backup e restauração

```bash
./scripts/backup.sh                       # cria backups/AAAA-MM-DD_HHMM/{banco.dump, arquivos.tar.gz}
./scripts/restaurar.sh backups/2026-07-14_2200
```

Detalhes e rotina recomendada em [deployment.md](deployment.md).

> ⚠️ `docker compose down` e `docker compose up --build` **preservam** os dados. `docker compose down -v` **apaga os volumes** e não deve ser usado em produção.
