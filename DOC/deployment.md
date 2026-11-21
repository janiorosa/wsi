# Implantação, backup e restauração

## Produção

1. Servidor Linux com Docker e Docker Compose.
2. `cp .env.exemplo .env`. Defina uma senha forte (somente letras e números) em `POSTGRES_PASSWORD`. Na rede local por HTTP, mantenha `WSI_COOKIE_SEGURO=false`.
3. **HTTPS (recomendado; a especificação o exige para acesso fora da rede local).** Coloque o sistema atrás do proxy reverso institucional (ou de um Caddy/nginx com certificado) apontando para `http://servidor:WSI_PORTA`, repassando `X-Forwarded-Proto` e `X-Real-IP`. Com `WSI_COOKIE_SEGURO=true`, o login só funciona por HTTPS.
4. `docker compose up -d --build`.
5. Faça o primeiro acesso do ADMIN (veja [running.md](running.md)), cadastre o critério geral e confira os dados institucionais.

## Atualização de versão

```bash
./scripts/backup.sh
git pull            # ou copie a nova versão
docker compose up -d --build
docker compose logs -f backend   # as migrações são aplicadas automaticamente
```

`docker compose build`, `up --build`, `down` e reinícios **não** apagam dados: o banco fica em `wsi_postgres_dados` e os arquivos em `wsi_arquivos`.

> ⚠️ **Nunca** execute `docker compose down -v` nem `docker volume rm wsi_…` em produção: isso apaga o banco e todos os PDFs, e nenhuma configuração impede a remoção explícita de volumes.

## Backup

`./scripts/backup.sh` cria `backups/AAAA-MM-DD_HHMM/` com:

- `banco.dump`: `pg_dump` em formato custom;
- `arquivos.tar.gz`: todo o volume de arquivos (artigos, apresentações, ATAs, declarações e publicações).

Os dois precisam ser guardados **juntos**: o banco referencia os arquivos pelas chaves. Recomendação: backup diário via cron, cópia para fora do servidor e retenção de 30 dias.

```cron
0 2 * * * cd /opt/wsi && ./scripts/backup.sh >> backups/backup.log 2>&1
```

## Restauração

```bash
./scripts/restaurar.sh backups/2026-07-14_0200
```

O script pede confirmação (digite `RESTAURAR`), para a aplicação, restaura o banco (`pg_restore --clean`) e os arquivos, ajusta as permissões e sobe tudo de novo. **Teste a restauração periodicamente** em outro servidor.

## Verificação pós-implantação

- `curl -fsS http://servidor:8080/api/v1/saude` → `{"status":"ok","banco":"ok"}`
- Gerar uma declaração anti-plágio de teste com uma conta de aluno.
- Conferir em *Fila de documentos* (ADMIN) que não há falhas.

## Monitoramento

- `docker compose ps`: o backend tem healthcheck.
- Os logs estruturados em JSON (`docker compose logs backend`) registram cada requisição, tarefa e erro.
- O painel ADMIN mostra as tarefas em processamento e as falhas dos últimos 30 dias.
