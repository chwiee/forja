# forja — branch de estudo

Este branch reconstrói o forja em 4 etapas, na ordem do *Manual da Forja*.
Cada etapa compila e tem os testes passando. Para ver exatamente o que uma
etapa acrescenta à anterior:

    git diff estudo-e1 estudo-e2

| Tag | Etapa | Capítulos do livro |
|---|---|---|
| `estudo-e1` | casca Cobra + consulta ao registry (`exists`, `inspect`) | 4, 5, 6, 8 |
| `estudo-e2` | motor buildah: `build`, `push`, `manifest`, multi-arch, imagem estática | 7, 9, 10, 13, 16 |
| `estudo-e3` | scan (Syft, Grype, Gitleaks), nota, gate e `run` | 11, 12, 14, 15 |
| `estudo-e4` | produção: `--registry`/ECR, CI, livro — igual à `main` | 17, 18, 19 |

## Etapa atual: e1 — a casca

O que tem: `forja exists` e `forja inspect` (funcionam no Windows nativo),
`forja version`, exit codes, injeção de dependência e testes com fake.

Validar:

    go vet -tags containers_image_openpgp ./...
    go test -tags containers_image_openpgp ./...
    go run -tags containers_image_openpgp ./cmd/forja exists docker.io/library/alpine:3.20

Esperado: `ok github.com/chwiee/forja/internal/cli` e `EXISTE  docker.io/library/alpine:3.20  sha256:...` (exit 0).
