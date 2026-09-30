# forja.ps1 - usa a imagem do forja no Windows como se fosse um comando local.
# Uso:  . .\scripts\forja.ps1      (carrega a função na sessão)
#       forja build -t localhost:5000/app:1.0 --push --tls-verify=false .
#       forja build --registry ecr --name org/app --tag v1.0.0 --push .
# A pasta atual vira /workspace dentro do container. As variáveis da AWS
# (AWS_*) e do forja (FORJA_ECR_*) da sessão são repassadas ao container.

$ForjaImage = if ($env:FORJA_IMAGE) { $env:FORJA_IMAGE } else { "ghcr.io/chwiee/forja:0.4.1" }
$ForjaCaps  = "SYS_ADMIN,CHOWN,DAC_OVERRIDE,FOWNER,FSETID,KILL,NET_BIND_SERVICE,SETFCAP,SETGID,SETPCAP,SETUID,SYS_CHROOT"
$ForjaEnv   = "AWS_REGION,AWS_ACCESS_KEY_ID,AWS_SECRET_ACCESS_KEY,AWS_SESSION_TOKEN,AWS_ENDPOINT_URL,FORJA_ECR_ACCOUNT,FORJA_ECR_REGION,FORJA_ECR_HOST"

function forja {
    $caps = @("--cap-drop", "ALL")
    foreach ($c in $ForjaCaps.Split(",")) { $caps += @("--cap-add", $c) }

    $net = @()
    if ($env:FORJA_NETWORK) { $net = @("--network", $env:FORJA_NETWORK) }

    # "-e NOME" sem valor: o Docker copia da sessão, e o segredo não aparece na linha de comando.
    $envs = @()
    foreach ($e in $ForjaEnv.Split(",")) {
        if (Test-Path "env:$e") { $envs += @("-e", $e) }
    }

    docker run --rm @caps @net @envs `
        -v "${PWD}:/workspace" `
        -v forja-tmp:/var/tmp `
        -v forja-storage:/var/lib/containers `
        -e STORAGE_DRIVER=overlay `
        $ForjaImage @args
}
