# forja.ps1 - usa a imagem do forja no Windows como se fosse um comando local.
# Uso:  . .\scripts\forja.ps1      (carrega a função na sessão)
#       forja build -t localhost:5000/app:1.0 --push --tls-verify=false .
# A pasta atual vira /workspace dentro do container.

$ForjaImage = if ($env:FORJA_IMAGE) { $env:FORJA_IMAGE } else { "forja:dev" }
$ForjaCaps  = "SYS_ADMIN,CHOWN,DAC_OVERRIDE,FOWNER,FSETID,KILL,NET_BIND_SERVICE,SETFCAP,SETGID,SETPCAP,SETUID,SYS_CHROOT"

function forja {
    $caps = @("--cap-drop", "ALL")
    foreach ($c in $ForjaCaps.Split(",")) { $caps += @("--cap-add", $c) }

    $net = @()
    if ($env:FORJA_NETWORK) { $net = @("--network", $env:FORJA_NETWORK) }

    docker run --rm @caps @net `
        -v "${PWD}:/workspace" `
        -v forja-tmp:/var/tmp `
        -v forja-storage:/var/lib/containers `
        -e STORAGE_DRIVER=overlay `
        $ForjaImage @args
}
