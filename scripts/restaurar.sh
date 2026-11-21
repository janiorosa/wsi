#!/usr/bin/env bash
# Restauração: ./scripts/restaurar.sh backups/AAAA-MM-DD_HHMM
# ATENÇÃO: substitui o banco e os arquivos atuais.
set -euo pipefail
cd "$(dirname "$0")/.."
source .env
ORIGEM="${1:?informe a pasta do backup}"
[ -f "$ORIGEM/banco.dump" ] && [ -f "$ORIGEM/arquivos.tar.gz" ] || { echo "Backup incompleto em $ORIGEM"; exit 1; }
read -r -p "Isto substituirá TODOS os dados atuais. Digite RESTAURAR para continuar: " CONFIRMA
[ "$CONFIRMA" = "RESTAURAR" ] || { echo "Cancelado."; exit 1; }
docker compose stop backend frontend
docker compose up -d postgres
echo "Restaurando banco..."
docker compose exec -T postgres pg_restore -U "${POSTGRES_USER:-wsi}" -d "${POSTGRES_DB:-wsi}" --clean --if-exists --no-owner < "$ORIGEM/banco.dump"
echo "Restaurando arquivos..."
docker run --rm -v wsi_arquivos:/dados -v "$PWD/$ORIGEM":/backup:ro alpine:3.20 sh -c "rm -rf /dados/* && tar xzf /backup/arquivos.tar.gz -C /dados && chown -R 10001:10001 /dados"
docker compose up -d
echo "Restauração concluída."
