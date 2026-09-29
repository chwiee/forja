package cli

import (
	"os"
	"strings"
	"testing"
)

// isola o teste: credenciais e registries do ambiente não podem vazar para cá.
func cleanRegistryEnv(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("REGISTRY_AUTH_FILE", "")
	t.Setenv("DOCKER_CONFIG", dir)
	t.Setenv("FORJA_REGISTRIES", "")
	t.Setenv("FORJA_ECR_ACCOUNT", "123456789012")
	t.Setenv("FORJA_ECR_REGION", "")
	t.Setenv("FORJA_ECR_HOST", "")
	t.Setenv("TMP", dir)    // os.TempDir no Windows
	t.Setenv("TMPDIR", dir) // os.TempDir no Linux
}

func TestPushWithRegistryGHCR(t *testing.T) {
	cleanRegistryEnv(t)
	eng := &fakeEngine{}
	_, code := run(t, &fakeRegistry{}, eng, "push", "--registry", "ghcr", "--name", "Chwiee/Forja", "--tag", "refs/tags/v1.2.0")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	if got, want := eng.gotPush.Image, "ghcr.io/chwiee/forja:v1.2.0"; got != want {
		t.Errorf("imagem = %q, queria %q", got, want)
	}
	if len(eng.credsFor) != 0 {
		t.Errorf("GHCR não deveria pedir credencial AWS; pediu para %v", eng.credsFor)
	}
}

func TestRunWithRegistryECR(t *testing.T) {
	cleanRegistryEnv(t)
	eng := &fakeEngine{}
	_, code := run(t, &fakeRegistry{}, eng, "run", "--registry", "ecr", "--name", "org/app", "-t", "refs/heads/feature/login")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	host := "123456789012.dkr.ecr.us-east-1.amazonaws.com"
	if got, want := eng.gotBuild.Image, host+"/org/app:feature-login"; got != want {
		t.Errorf("imagem = %q, queria %q", got, want)
	}
	if len(eng.credsFor) != 1 || eng.credsFor[0] != host {
		t.Errorf("a credencial do ECR deveria ser pedida uma vez para %s; pedidos: %v", host, eng.credsFor)
	}
	data, err := os.ReadFile(os.Getenv("REGISTRY_AUTH_FILE"))
	if err != nil || !strings.Contains(string(data), host) {
		t.Errorf("REGISTRY_AUTH_FILE deveria ter a credencial do ECR: %s, %v", data, err)
	}
}

func TestFullECRReferenceAlsoGetsCredentials(t *testing.T) {
	cleanRegistryEnv(t)
	eng := &fakeEngine{}
	_, code := run(t, &fakeRegistry{}, eng, "push", "999999999999.dkr.ecr.sa-east-1.amazonaws.com/app:1")
	if code != ExitOK || len(eng.credsFor) != 1 {
		t.Errorf("exit = %d, credsFor = %v; -t com endereço de ECR também precisa de credencial", code, eng.credsFor)
	}
}

func TestManifestWithRegistryUsesTags(t *testing.T) {
	cleanRegistryEnv(t)
	eng := &fakeEngine{}
	_, code := run(t, &fakeRegistry{}, eng, "manifest", "--registry", "ghcr", "--name", "org/app", "v1", "v1-amd64", "v1-arm64")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	if eng.gotManifest.Name != "ghcr.io/org/app:v1" || eng.gotManifest.Images[1] != "ghcr.io/org/app:v1-arm64" {
		t.Errorf("manifest = %+v", eng.gotManifest)
	}
}

func TestRegistryFlagErrors(t *testing.T) {
	cleanRegistryEnv(t)
	tests := []struct {
		name    string
		args    []string
		wantOut string
	}{
		{"--name sem --registry", []string{"push", "--name", "org/app", "--tag", "v1"}, "exigem --registry"},
		{"endereço e --registry juntos", []string{"push", "--registry", "ghcr", "ghcr.io/org/app:v1"}, "use --name e --tag"},
		{"registry desconhecido", []string{"push", "--registry", "quay", "--name", "a", "--tag", "b"}, "desconhecido"},
		{"--registry sem --name", []string{"build", "--registry", "ecr", "-t", "v1"}, "informe --name"},
		{"push sem nada", []string{"push"}, "informe a IMAGEM"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, code := run(t, &fakeRegistry{}, &fakeEngine{}, tt.args...)
			if code != ExitFailure || !strings.Contains(out, tt.wantOut) {
				t.Errorf("exit = %d, saída = %q; queria erro com %q", code, out, tt.wantOut)
			}
		})
	}
}
