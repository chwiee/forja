#!/usr/bin/env sh
# Roda o forja num container com as permissões mínimas (Manual da Forja: "Permissões mínimas").
# A pasta atual vira /workspace; as credenciais do "docker login" são reusadas.
#
#   ci/forja.sh run -t ghcr.io/org/app:1.0 .
#   FORJA_IMAGE=ghcr.io/chwiee/forja:0.3.0 ci/forja.sh scan --remote ghcr.io/org/app:1.0
#   FORJA_DOCKER_OPTS="-e AWS_REGION=us-east-1" ci/forja.sh run --registry ecr --name org/app -t v1 .
set -eu

AUTH="$HOME/.docker/config.json"
[ -f "$AUTH" ] || { mkdir -p "$HOME/.docker"; echo '{}' > "$AUTH"; }

exec docker run --rm \
  --cap-drop ALL \
  --cap-add SYS_ADMIN \
  --cap-add CHOWN --cap-add DAC_OVERRIDE --cap-add FOWNER --cap-add FSETID \
  --cap-add KILL --cap-add NET_BIND_SERVICE --cap-add SETFCAP --cap-add SETGID \
  --cap-add SETPCAP --cap-add SETUID --cap-add SYS_CHROOT \
  --security-opt apparmor=unconfined \
  --network host \
  -v "$PWD:/workspace" \
  -v forja-tmp:/var/tmp \
  -v "$AUTH:/auth/config.json:ro" \
  -e REGISTRY_AUTH_FILE=/auth/config.json \
  -e DOCKER_CONFIG=/auth \
  -e AWS_REGION -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_SESSION_TOKEN \
  -e AWS_ENDPOINT_URL -e FORJA_ECR_ACCOUNT -e FORJA_ECR_REGION -e FORJA_ECR_HOST \
  ${FORJA_DOCKER_OPTS:-} \
  "${FORJA_IMAGE:-ghcr.io/chwiee/forja:latest}" "$@"
