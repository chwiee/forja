# forja

CLI de CI que builda, escaneia e publica imagens de container a partir do
Dockerfile da aplicação, sem Docker nem daemon: usa a biblioteca do buildah
dentro do próprio processo. A imagem só é publicada se passar no gate de
segurança (CVEs, secrets, configuração), com nota de 0 a 100.

```
forja run -t ghcr.io/org/app:1.0 --immutable --sarif forja.sarif .
  [1/5] exists   a tag ainda não existe
  [2/5] build    Dockerfile → imagem (uma ou várias arquiteturas)
  [3/5] scan     Syft + Grype (CVEs), Gitleaks (secrets), ENV/ARG/USER
  [4/5] gate     reprovada: exit 3 e nada é publicado
  [5/5] push     aprovada: publica e imprime o digest
```

- Binário estático (cgo + musl): roda em qualquer imagem Linux, amd64 e arm64.
- Imagem oficial multi-arch: `ghcr.io/chwiee/forja`.
- Permissões mínimas: root no container + 12 capabilities (inclui `SYS_ADMIN`).
- Documentação completa: *Manual da Forja*.

## Uso rápido

```sh
docker login ghcr.io
FORJA_IMAGE=ghcr.io/chwiee/forja:latest ci/forja.sh run -t ghcr.io/org/app:1.0 .
```

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

Exit codes: `0` ok · `1` erro · `2` tag inexistente/já existente · `3` reprovada no gate.

## Desenvolvimento

```sh
go test -tags containers_image_openpgp ./...
docker build -t forja:dev .
```

O CI (`.github/workflows/ci.yml`) testa, publica a imagem e roda o forja de
verdade em runners amd64 e arm64 e num pod do Kubernetes (kind).
