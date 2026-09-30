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

## Etapa atual: e3 — scan, nota e gate

O que tem: `forja scan` (Syft + Grype para CVEs, Gitleaks para secrets em todas
as camadas e no histórico, ENV/ARG/USER), a política com pesos, tetos e
exceções (`internal/scan/defaults/policy.yaml`), saída em tabela/JSON/SARIF, e
`forja run`: exists → build → scan → gate → push. Imagens-isca em `testdata/iscas`.

Validar (com o registry `lab` da etapa 2):

    go test -tags containers_image_openpgp ./...
    docker build -t forja:e3 .
    FORJA="docker run --rm --cap-drop ALL --cap-add SYS_ADMIN --cap-add CHOWN --cap-add DAC_OVERRIDE 
      --cap-add FOWNER --cap-add FSETID --cap-add KILL --cap-add NET_BIND_SERVICE --cap-add SETFCAP 
      --cap-add SETGID --cap-add SETPCAP --cap-add SETUID --cap-add SYS_CHROOT 
      --network lab -v $PWD:/workspace -v e3-tmp:/var/tmp forja:e3"
    $FORJA run -t registry:5000/estudo/e3:1 --tls-verify=false testdata/multi
    TOKEN="ghp_$(head -c 300 /dev/urandom | tr -dc A-Za-z0-9 | head -c 36)"
    $FORJA run -t registry:5000/estudo/e3:secret --build-arg "API_TOKEN=$TOKEN" --tls-verify=false testdata/iscas/secret

Esperado: a primeira APROVADA com `PUSH OK` (exit 0); a segunda REPROVADA com
`REPROVOU: secret encontrado` e `nada foi publicado` (exit 3).

Use um token ALEATÓRIO: o Gitleaks ignora tokens de baixa entropia
(`ghp_abcdef...0123456789` passa sem ser acusado como secret).
