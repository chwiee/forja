# forja

CLI de CI que builda, escaneia e publica imagens de container a partir do
Dockerfile da aplicação, sem Docker nem daemon: usa as bibliotecas do buildah,
Syft, Grype e Gitleaks dentro do próprio processo. A imagem só é publicada se
passar no gate de segurança (CVEs, secrets, configuração), com nota de 0 a 100.

```
forja run --registry ecr --name ${{ github.repository }} --tag ${{ github.ref }} --immutable .
  [1/5] exists   a tag ainda não existe
  [2/5] build    Dockerfile → imagem (uma ou várias arquiteturas)
  [3/5] scan     Syft + Grype (CVEs), Gitleaks (secrets), ENV/ARG/USER
  [4/5] gate     reprovada: exit 3 e nada é publicado
  [5/5] push     aprovada: publica e imprime o digest
```

- Binário estático (cgo + musl): roda em qualquer imagem Linux, amd64 e arm64.
- Imagem oficial multi-arch e assinada (cosign): `ghcr.io/chwiee/forja`.
- Registries por nome: `--registry ecr` (conta central, us-east-1) ou `--registry ghcr`.
- Credencial do ECR pelo SDK da AWS: Pod Identity no EKS, OIDC no GitHub Actions.
- Permissões mínimas: root no container + 12 capabilities (inclui `SYS_ADMIN`).
- Documentação completa: *Manual da Forja* (fonte em `docs/livro/`).

## Uso rápido

```sh
docker login ghcr.io
FORJA_IMAGE=ghcr.io/chwiee/forja:0.4.1 ci/forja.sh run --registry ghcr --name org/app --tag v1.0.0 .
```

ECR: defina `FORJA_ECR_ACCOUNT` (conta central) e tenha credencial AWS no
ambiente (Pod Identity, OIDC ou variáveis `AWS_*`). Exemplos prontos em
`examples/github-actions/` e `deploy/k8s-pod.yaml`.

No Windows (PowerShell): `. .\scripts\forja.ps1` e depois `forja run ...`.

## Comandos

| Comando | O que faz |
|---|---|
| `run` | exists → build → scan → gate → push |
| `build` | builda (`--platform linux/amd64,linux/arm64` gera manifest list) |
| `scan` | nota de segurança de uma imagem local ou `--remote` |
| `push` | publica (`--immutable` recusa tag existente) |
| `manifest` | junta imagens de arquiteturas diferentes num nome |
| `exists`, `inspect` | consultas ao registry (funcionam no Windows nativo) |

Todos aceitam `--registry NOME --name REPO --tag TAG` no lugar do endereço
completo. `--tag` aceita `refs/tags/v1.2.0` (vira `v1.2.0`) e
`refs/heads/feature/x` (vira `feature-x`).

Exit codes: `0` ok · `1` erro · `2` tag inexistente/já existente · `3` reprovada no gate.

## Configuração

| Variável | Uso |
|---|---|
| `FORJA_ECR_ACCOUNT`, `FORJA_ECR_REGION`, `FORJA_ECR_HOST` | conta, região e host do ECR |
| `FORJA_REGISTRIES` | arquivo de registries próprio (padrão: `internal/registries/defaults/registries.yaml`) |
| `FORJA_DB_DIR` | cache do banco de CVEs (padrão: `/var/tmp/forja-db`) |
| `STORAGE_DRIVER` | `vfs` (padrão) ou `overlay` |

## Desenvolvimento

```sh
go test -tags containers_image_openpgp ./...
docker build -t forja:dev .
```

O CI (`.github/workflows/ci.yml`) tem 7 jobs: testes, imagem multi-arch
assinada, e2e em runners amd64 e arm64 nativos, manifest, pod no Kubernetes
(kind) e ECR emulado (floci).
