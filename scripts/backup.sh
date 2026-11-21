#!/usr/bin/env bash
# Backup completo: banco (pg_dump) + arquivos (volume wsi_arquivos).
# Uso: ./scripts/backup.sh  → cria backups/AAAA-MM-DD_HHMM/
set -euo pipefail
cd "$(dirname "$0")/.."
source .env
DESTINO="backups/$(date +%Y-%m-%d_%H%M)"
mkdir -p "$DESTINO"
echo "Exportando banco..."
docker compose exec -T postgres pg_dump -U "${POSTGRES_USER:-wsi}" -d "${POSTGRES_DB:-wsi}" --format=custom > "$DESTINO/banco.dump"
echo "Copiando arquivos..."
docker run --rm -v wsi_arquivos:/dados:ro -v "$PWD/$DESTINO":/backup alpine:3.20 tar czf /backup/arquivos.tar.gz -C /dados .
echo "Backup concluído em $DESTINO"
ls -lh "$DESTINO"
