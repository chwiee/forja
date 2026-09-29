package registries

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeTag(t *testing.T) {
	tests := []struct{ in, want string }{
		{"refs/tags/v1.2.0", "v1.2.0"},
		{"v1.2.0", "v1.2.0"},
		{"refs/heads/main", "main"},
		{"refs/heads/feature/login", "feature-login"},
		{"refs/pull/42/merge", "pr-42"},
		{"feat@2026 #1", "feat-2026-1"},
		{"-.lixo", "lixo"},
		{strings.Repeat("a", 200), strings.Repeat("a", 128)},
	}
	for _, tt := range tests {
		got, err := NormalizeTag(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("NormalizeTag(%q) = %q, %v; queria %q", tt.in, got, err, tt.want)
		}
	}
	if _, err := NormalizeTag("refs/heads/"); err == nil {
		t.Error("tag vazia deveria dar erro")
	}
}

func TestNormalizeName(t *testing.T) {
	if got, _ := NormalizeName("Chwiee/Forja"); got != "chwiee/forja" {
		t.Errorf("queria minúsculas, veio %q", got)
	}
	for _, bad := range []string{"", "org/Repo Com Espaço", "org//repo", "-org/repo"} {
		if _, err := NormalizeName(bad); err == nil {
			t.Errorf("NormalizeName(%q) deveria falhar", bad)
		}
	}
}

func TestLookupECR(t *testing.T) {
	t.Setenv("FORJA_ECR_ACCOUNT", "123456789012")
	regs, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Lookup(regs, "ecr")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := r.Reference("Org/App", "refs/tags/v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if want := "123456789012.dkr.ecr.us-east-1.amazonaws.com/org/app:v1.0.0"; ref != want {
		t.Errorf("ref = %q, queria %q", ref, want)
	}
}

func TestLookupErrors(t *testing.T) {
	t.Setenv("FORJA_ECR_ACCOUNT", "")
	regs, _ := Load("")
	if _, err := Lookup(regs, "ecr"); err == nil || !strings.Contains(err.Error(), "FORJA_ECR_ACCOUNT") {
		t.Errorf("sem conta, o erro deveria dizer como configurar; veio %v", err)
	}
	if _, err := Lookup(regs, "quay"); err == nil || !strings.Contains(err.Error(), "ecr, ghcr") {
		t.Errorf("registry desconhecido deveria listar os conhecidos; veio %v", err)
	}
}

func TestLoadFileAndHostOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.yaml")
	os.WriteFile(path, []byte("ecr:\n  type: ecr\n  account: \"111111111111\"\n  region: sa-east-1\n"), 0o644)
	t.Setenv("FORJA_ECR_HOST", "000000000000.dkr.ecr.us-east-1.localhost:4566")
	regs, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := Lookup(regs, "ecr")
	if r.Host != "000000000000.dkr.ecr.us-east-1.localhost:4566" || r.Region != "sa-east-1" {
		t.Errorf("registry = %+v", r)
	}
}

func TestDecodeToken(t *testing.T) {
	c, err := decodeToken(base64.StdEncoding.EncodeToString([]byte("AWS:segredo")))
	if err != nil || c.Username != "AWS" || c.Password != "segredo" {
		t.Errorf("decodeToken = %+v, %v", c, err)
	}
	if _, err := decodeToken("não-é-base64"); err == nil {
		t.Error("token inválido deveria dar erro")
	}
}

func TestInstallAuthKeepsExistingCredentials(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "old.json")
	os.WriteFile(existing, []byte(`{"auths":{"ghcr.io":{"auth":"Z2g6dG9rZW4="}}}`), 0o600)
	t.Setenv("REGISTRY_AUTH_FILE", existing)
	t.Setenv("DOCKER_CONFIG", "")

	path, err := InstallAuth(filepath.Join(dir, "novo"), "123456789012.dkr.ecr.us-east-1.amazonaws.com", &Credentials{"AWS", "senha"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	s := string(data)
	if !strings.Contains(s, `"ghcr.io"`) || !strings.Contains(s, "123456789012.dkr.ecr.us-east-1.amazonaws.com") {
		t.Errorf("o arquivo precisa ter a credencial antiga e a nova: %s", s)
	}
	if os.Getenv("REGISTRY_AUTH_FILE") != path || os.Getenv("DOCKER_CONFIG") != filepath.Dir(path) {
		t.Error("REGISTRY_AUTH_FILE e DOCKER_CONFIG precisam apontar para o arquivo novo")
	}
}

func TestDetectECR(t *testing.T) {
	r, ok := DetectECR("123456789012.dkr.ecr.sa-east-1.amazonaws.com")
	if !ok || r.Account != "123456789012" || r.Region != "sa-east-1" {
		t.Errorf("DetectECR = %+v, %v", r, ok)
	}
	if _, ok := DetectECR("ghcr.io"); ok {
		t.Error("ghcr.io não é ECR")
	}
}
