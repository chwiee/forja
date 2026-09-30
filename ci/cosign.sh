#!/usr/bin/env sh
# Roda o cosign (container oficial) com as credenciais do "docker login".
# A chave privada e a senha chegam por variável de ambiente (secrets do CI).
set -eu
exec docker run --rm --user "$(id -u)" \
  -v "$HOME/.docker:/dc:ro" -e DOCKER_CONFIG=/dc \
  -v "$PWD:/w" -w /w \
  -e COSIGN_PRIVATE_KEY -e COSIGN_PASSWORD \
  ghcr.io/sigstore/cosign/cosign:v3.1.3 "$@"
