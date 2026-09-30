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

## Etapa atual: e2 — o motor buildah

O que tem: `forja build` (uma ou várias arquiteturas com `--platform`),
`push`, `manifest`, a interface de rede própria (`hostnet_linux.go`), o limite
de arquivos correto para os RUN (`currentUlimits`), a configuração embutida
(`policy.json`, `registries.conf`) e a imagem estática multi-arch (`Dockerfile`).
No Windows, `build` e `push` recusam com mensagem clara: rode a imagem.

Validar:

    go test -tags containers_image_openpgp ./...
    docker build -t forja:e2 .
    docker network create lab
    docker run -d --name registry --network lab registry:3
    docker run --rm --cap-drop ALL --cap-add SYS_ADMIN --cap-add CHOWN --cap-add DAC_OVERRIDE \
      --cap-add FOWNER --cap-add FSETID --cap-add KILL --cap-add NET_BIND_SERVICE --cap-add SETFCAP \
      --cap-add SETGID --cap-add SETPCAP --cap-add SETUID --cap-add SYS_CHROOT \
      --network lab -v "$PWD:/workspace" -v e2-tmp:/var/tmp \
      forja:e2 build -t registry:5000/estudo/e2:1 --push --tls-verify=false testdata/multi

Esperado: `BUILD OK  registry:5000/estudo/e2:1` e `PUSH OK ... sha256:...`.
