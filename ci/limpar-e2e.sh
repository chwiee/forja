#!/usr/bin/env bash
# Apaga do GHCR as imagens de TESTE do CI (tags e2e-*) com mais de N dias.
#
#   ci/limpar-e2e.sh             # modo seco: só lista o que apagaria
#   ci/limpar-e2e.sh --aplicar   # apaga de verdade
#
# Travas: só apaga uma versão se TODAS as tags dela começarem com "e2e-".
# Versões com qualquer outra tag (0.4.1, latest, assinaturas .sig) e versões
# sem tag (camadas de manifest lists de release) nunca são tocadas.
set -euo pipefail

OWNER="${OWNER:-chwiee}"
PACKAGE="${PACKAGE:-forja}"
DIAS="${DIAS:-7}"
APLICAR=false
[ "${1:-}" = "--aplicar" ] && APLICAR=true

limite=$(date -u -d "-${DIAS} days" +%Y-%m-%dT%H:%M:%SZ)
echo "pacote ghcr.io/$OWNER/$PACKAGE; apagando e2e-* criadas antes de $limite (aplicar=$APLICAR)"

gh api --paginate "/users/$OWNER/packages/container/$PACKAGE/versions" \
  --jq ".[] | select(.created_at < \"$limite\")
            | select((.metadata.container.tags | length) > 0)
            | select(all(.metadata.container.tags[]; startswith(\"e2e-\")))
            | \"\(.id)\t\(.created_at)\t\(.metadata.container.tags | join(\",\"))\"" > alvos.txt

echo "versões a apagar: $(wc -l < alvos.txt)"
while IFS=$'\t' read -r id criado tags; do
  echo "  $id  $criado  $tags"
  if $APLICAR; then
    gh api -X DELETE "/users/$OWNER/packages/container/$PACKAGE/versions/$id" > /dev/null
  fi
done < alvos.txt
$APLICAR || echo "modo seco: nada foi apagado (use --aplicar)"
