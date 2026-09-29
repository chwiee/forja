//go:build linux

package build

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

// Configuração padrão embutida no binário. Assim o forja funciona copiado para
// qualquer imagem, mesmo sem /etc/containers. Se a imagem tiver os arquivos,
// eles vencem: o administrador pode sobrescrever sem recompilar.
//
//go:embed defaults/policy.json defaults/registries.conf
var defaults embed.FS

// configPaths devolve os caminhos de policy.json e registries.conf a usar.
// String vazia significa "use o caminho padrão do sistema".
func configPaths() (policy, registries string, err error) {
	policy, err = pick("/etc/containers/policy.json", "defaults/policy.json")
	if err != nil {
		return "", "", err
	}
	registries, err = pick("/etc/containers/registries.conf", "defaults/registries.conf")
	return policy, registries, err
}

// pick usa o arquivo do sistema se existir; senão, grava o embutido em
// /var/tmp (que o forja já exige gravável) e devolve esse caminho.
func pick(systemPath, embedded string) (string, error) {
	if _, err := os.Stat(systemPath); err == nil {
		return "", nil
	}
	data, err := defaults.ReadFile(embedded)
	if err != nil {
		return "", err
	}
	dir := filepath.Join("/var/tmp", "forja-defaults")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("criando %s: %w", dir, err)
	}
	path := filepath.Join(dir, filepath.Base(embedded))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("gravando %s: %w", path, err)
	}
	return path, nil
}
