package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/chwiee/forja/internal/registry"
)

// fakeClient implementa registry.Client sem tocar na rede.
type fakeClient struct {
	digests map[string]string
	info    *registry.ImageInfo
}

func (f *fakeClient) Digest(_ context.Context, image string) (string, error) {
	if d, ok := f.digests[image]; ok {
		return d, nil
	}
	return "", registry.ErrNotFound
}

func (f *fakeClient) Inspect(context.Context, string) (*registry.ImageInfo, error) {
	return f.info, nil
}

// run executa o CLI como se fosse o terminal e devolve saída + exit code.
func run(t *testing.T, fake *fakeClient, args ...string) (string, int) {
	t.Helper()
	cmd := NewRootCmd(func(registry.Config) registry.Client { return fake })
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)

	err := cmd.Execute()
	var exitErr *ExitError
	switch {
	case err == nil:
		return out.String(), ExitOK
	case errors.As(err, &exitErr):
		return out.String(), exitErr.Code
	default:
		return out.String() + err.Error(), ExitFailure
	}
}

func TestExists(t *testing.T) {
	fake := &fakeClient{digests: map[string]string{"reg/app:1.0": "sha256:abc"}}

	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
	}{
		{"tag existe", []string{"exists", "reg/app:1.0"}, ExitOK, "EXISTE"},
		{"tag não existe", []string{"exists", "reg/app:2.0"}, ExitNotFound, "NÃO EXISTE"},
		{"saída json", []string{"exists", "reg/app:1.0", "-o", "json"}, ExitOK, `"exists":true`},
		{"output inválido", []string{"exists", "reg/app:1.0", "-o", "yaml"}, ExitFailure, "--output deve ser"},
		{"sem argumento", []string{"exists"}, ExitFailure, "accepts 1 arg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, code := run(t, fake, tt.args...)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, queria %d (saída: %s)", code, tt.wantCode, out)
			}
			if !strings.Contains(out, tt.wantOut) {
				t.Errorf("saída %q não contém %q", out, tt.wantOut)
			}
		})
	}
}

func TestInspectFindsHardcodedSecret(t *testing.T) {
	fake := &fakeClient{info: &registry.ImageInfo{
		User: "app",
		Env:  []string{"PATH=/bin", "DB_PASSWORD=supersecreta"},
	}}

	out, code := run(t, fake, "inspect", "reg/app:1.0", "--fail-on-findings")

	if code != ExitFindings {
		t.Fatalf("exit code = %d, queria %d", code, ExitFindings)
	}
	if strings.Contains(out, "supersecreta") {
		t.Error("o valor do segredo vazou na saída")
	}
	if !strings.Contains(out, "DB_PASSWORD=****") {
		t.Errorf("esperava a variável mascarada, saída: %s", out)
	}
}
