#!/usr/bin/env bash
# Gera o Manual da Forja a partir dos capítulos e do código REAL do repositório.
#
#   docs/livro/build.sh            → docs/livro/dist/manual-da-forja.html
#
# Nos capítulos:
#   @@arquivo caminho@@   conteúdo de um arquivo do repositório (caminho a partir da raiz)
#   @@saida nome@@        log de uma execução real, guardado em docs/livro/saidas/
# O conteúdo entra escapado para HTML. Se um arquivo citado não existir, o build falha:
# o livro nunca mostra código que não está no repositório.
set -euo pipefail
cd "$(dirname "$0")"
ROOT="$(cd ../.. && pwd)"
mkdir -p dist

# 1. junta estilo + capítulos na ordem + script
{
  cat _estilo.html
  while read -r cap; do [ -n "$cap" ] && cat "$cap"; done < ordem.txt
  cat _script.html
} > dist/pagina.html

# 2. resolve as inclusões
awk -v root="$ROOT" '
function incluir(caminho,   linha, primeira, n) {
  if ((getline linha < caminho) < 0) { print "FALTA: " caminho > "/dev/stderr"; erro = 1; return }
  close(caminho)
  primeira = 1
  while ((getline linha < caminho) > 0) {
    sub(/\r$/, "", linha)
    gsub(/&/, "\\&amp;", linha); gsub(/</, "\\&lt;", linha); gsub(/>/, "\\&gt;", linha)
    if (!primeira) printf "\n"
    printf "%s", linha; primeira = 0
  }
  close(caminho)
}
{
  while (match($0, /@@(arquivo|saida) [^@]+@@/)) {
    antes = substr($0, 1, RSTART - 1)
    marca = substr($0, RSTART + 2, RLENGTH - 4)
    depois = substr($0, RSTART + RLENGTH)
    split(marca, p, " ")
    caminho = (p[1] == "arquivo") ? root "/" p[2] : "saidas/" p[2]
    printf "%s", antes
    incluir(caminho)
    $0 = depois
  }
  print
}
END { if (erro) exit 1 }
' dist/pagina.html > dist/corpo.html

# 3. versão para abrir direto no navegador (com cabeçalho HTML e o mermaid)
{
  printf '<!doctype html>\n<html lang="pt-BR">\n<head>\n<meta charset="utf-8">\n'
  printf '<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">\n'
  sed 's#</style>#</style>\n</head>\n<body>#' dist/corpo.html
  printf '<script type="module">import mermaid from "https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs"; mermaid.initialize({startOnLoad:true});</script>\n'
  printf '</body>\n</html>\n'
} > dist/manual-da-forja.html

# dist/corpo.html é a versão para publicar como Artifact (o visualizador põe o cabeçalho).
rm -f dist/pagina.html
echo "ok: dist/manual-da-forja.html ($(wc -c < dist/manual-da-forja.html) bytes)"
