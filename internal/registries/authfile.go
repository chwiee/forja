package registries

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// InstallAuth grava num arquivo as credenciais já existentes (docker login /
// REGISTRY_AUTH_FILE) mais a do host informado, e aponta REGISTRY_AUTH_FILE
// (containers/image, buildah) e DOCKER_CONFIG (Syft) para ele.
//
// Não dá para usar SystemContext.DockerAuthConfig: ele vale para TODO registry,
// e o pull anônimo do FROM no Docker Hub quebraria com a senha do ECR.
func InstallAuth(dir, host string, c *Credentials) (string, error) {
	auths := map[string]map[string]string{}
	if existing := currentAuthFile(); existing != "" {
		if data, err := os.ReadFile(existing); err == nil {
			var cfg struct {
				Auths map[string]map[string]string `json:"auths"`
			}
			if json.Unmarshal(data, &cfg) == nil && cfg.Auths != nil {
				auths = cfg.Auths
			}
		}
	}
	auths[host] = map[string]string{
		"auth": base64.StdEncoding.EncodeToString([]byte(c.Username + ":" + c.Password)),
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "config.json") // nome que o DOCKER_CONFIG exige
	data, _ := json.Marshal(map[string]any{"auths": auths})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("gravando credenciais: %w", err)
	}
	os.Setenv("REGISTRY_AUTH_FILE", path)
	os.Setenv("DOCKER_CONFIG", dir)
	return path, nil
}

// currentAuthFile é o arquivo de credenciais em uso, na ordem do containers/image.
func currentAuthFile() string {
	if p := os.Getenv("REGISTRY_AUTH_FILE"); p != "" {
		return p
	}
	if d := os.Getenv("DOCKER_CONFIG"); d != "" {
		return filepath.Join(d, "config.json")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".docker", "config.json")
	}
	return ""
}

var ecrHost = regexp.MustCompile(`^(\d{12})\.dkr\.ecr\.([a-z0-9-]+)\.amazonaws\.com$`)

// DetectECR reconhece um host de ECR, para quem usa -t com o endereço
// completo em vez de --registry ecr.
func DetectECR(host string) (Registry, bool) {
	m := ecrHost.FindStringSubmatch(host)
	if m == nil {
		return Registry{}, false
	}
	return Registry{Name: "ecr", Type: TypeECR, Host: host, Account: m[1], Region: m[2]}, true
}
