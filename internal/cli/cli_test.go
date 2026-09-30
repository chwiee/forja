package cli

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chwiee/forja/internal/build"
	"github.com/chwiee/forja/internal/registry"
)

// fakeRegistry implementa registry.Client sem tocar na rede.
type fakeRegistry struct {
	digests map[string]string
	info    *registry.ImageInfo
}

func (f *fakeRegistry) Digest(_ context.Context, image string) (string, error) {
	if d, ok := f.digests[image]; ok {
		return d, nil
	}
	return "", registry.ErrNotFound
}

func (f *fakeRegistry) Inspect(context.Context, string) (*registry.ImageInfo, error) {
	return f.info, nil
}

// fakeEngine implementa build.Engine e guarda o que recebeu.
type fakeEngine struct {
	gotBuild    build.Options
	gotPush     build.PushOptions
	gotManifest build.ManifestOptions
	pushed      bool
	buildErr    error
}

func (f *fakeEngine) Build(_ context.Context, o build.Options) (string, error) {
	f.gotBuild = o
	return "0123456789abcdef", f.buildErr
}

func (f *fakeEngine) Push(_ context.Context, o build.PushOptions) (string, error) {
	f.gotPush, f.pushed = o, true
	return "sha256:feed", nil
}

func (f *fakeEngine) Manifest(_ context.Context, o build.ManifestOptions) (string, error) {
	f.gotManifest = o
	return "sha256:list", nil
}

func TestBuildMultiArch(t *testing.T) {
	eng := &fakeEngine{}
	_, code := run(t, &fakeRegistry{}, eng, "build", "-t", "reg/app:1", "--platform", "linux/amd64,linux/arm64/v8")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	want := []build.Platform{{OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64", Variant: "v8"}}
	if len(eng.gotBuild.Platforms) != 2 || eng.gotBuild.Platforms[0] != want[0] || eng.gotBuild.Platforms[1] != want[1] {
		t.Errorf("Platforms = %v, queria %v", eng.gotBuild.Platforms, want)
	}

	_, code = run(t, &fakeRegistry{}, &fakeEngine{}, "build", "-t", "reg/app:1", "--platform", "arm64")
	if code != ExitFailure {
		t.Errorf("plataforma sem os/: exit = %d, queria %d", code, ExitFailure)
	}
}

func TestManifest(t *testing.T) {
	eng := &fakeEngine{}
	out, code := run(t, &fakeRegistry{}, eng, "manifest", "reg/app:1", "reg/app:1-amd64", "reg/app:1-arm64")
	if code != ExitOK {
		t.Fatalf("exit = %d, saída: %s", code, out)
	}
	if eng.gotManifest.Name != "reg/app:1" || len(eng.gotManifest.Images) != 2 {
		t.Errorf("manifest recebido: %+v", eng.gotManifest)
	}
	if _, code := run(t, &fakeRegistry{}, &fakeEngine{}, "manifest", "reg/app:1"); code != ExitFailure {
		t.Errorf("sem imagens: exit = %d, queria %d", code, ExitFailure)
	}
}

// run executa o CLI como se fosse o terminal e devolve saída + exit code.
func run(t *testing.T, reg *fakeRegistry, eng *fakeEngine, args ...string) (string, int) {
	t.Helper()
	cmd := NewRootCmd(Deps{
		NewClient: func(registry.Config) registry.Client { return reg },
		Engine:    eng,
	})
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

func TestBuildPassesOptionsToEngine(t *testing.T) {
	eng := &fakeEngine{}
	out, code := run(t, &fakeRegistry{}, eng,
		"build", "-t", "reg/app:1", "--build-arg", "VERSION=1.0", "--storage-driver", "overlay", "app")

	if code != ExitOK {
		t.Fatalf("exit = %d, saída: %s", code, out)
	}
	if got, want := eng.gotBuild.Containerfile, filepath.Join("app", "Dockerfile"); got != want {
		t.Errorf("Containerfile = %q, queria %q (relativo ao contexto)", got, want)
	}
	if eng.gotBuild.BuildArgs["VERSION"] != "1.0" {
		t.Errorf("build-arg não chegou ao engine: %v", eng.gotBuild.BuildArgs)
	}
	if eng.gotBuild.StorageDriver != "overlay" {
		t.Errorf("storage driver = %q", eng.gotBuild.StorageDriver)
	}
	if eng.pushed {
		t.Error("não deveria fazer push sem --push")
	}
	if !strings.Contains(out, "BUILD OK  reg/app:1  id=0123456789ab") {
		t.Errorf("saída inesperada: %s", out)
	}
}

func TestBuildWithPush(t *testing.T) {
	eng := &fakeEngine{}
	out, code := run(t, &fakeRegistry{}, eng, "build", "-t", "reg/app:1", "--push", "-o", "json")
	if code != ExitOK || !eng.pushed {
		t.Fatalf("exit = %d, pushed = %v", code, eng.pushed)
	}
	if !strings.Contains(out, `"digest":"sha256:feed"`) {
		t.Errorf("JSON sem digest: %s", out)
	}
}

func TestBuildErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		engine  *fakeEngine
		wantOut string
	}{
		{"sem --tag", []string{"build"}, &fakeEngine{}, `"tag" not set`},
		{"build-arg inválido", []string{"build", "-t", "x", "--build-arg", "SEMIGUAL"}, &fakeEngine{}, "CHAVE=VALOR"},
		{"erro do engine", []string{"build", "-t", "x"}, &fakeEngine{buildErr: build.ErrUnsupported}, "só funcionam em Linux"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, code := run(t, &fakeRegistry{}, tt.engine, tt.args...)
			if code != ExitFailure {
				t.Errorf("exit = %d, queria %d", code, ExitFailure)
			}
			if !strings.Contains(out, tt.wantOut) {
				t.Errorf("saída %q não contém %q", out, tt.wantOut)
			}
		})
	}
}

func TestPushImmutable(t *testing.T) {
	reg := &fakeRegistry{digests: map[string]string{"reg/app:1": "sha256:old"}}

	eng := &fakeEngine{}
	_, code := run(t, reg, eng, "push", "reg/app:1", "--immutable")
	if code != ExitNotFound || eng.pushed {
		t.Errorf("tag existente: exit = %d, pushed = %v; queria exit 2 sem push", code, eng.pushed)
	}

	eng = &fakeEngine{}
	_, code = run(t, reg, eng, "push", "reg/app:2", "--immutable")
	if code != ExitOK || !eng.pushed {
		t.Errorf("tag nova: exit = %d, pushed = %v; queria exit 0 com push", code, eng.pushed)
	}
}

func TestInspectMasksSecrets(t *testing.T) {
	reg := &fakeRegistry{info: &registry.ImageInfo{User: "app", Env: []string{"DB_PASSWORD=supersecreta"}}}
	out, code := run(t, reg, &fakeEngine{}, "inspect", "reg/app:1", "--fail-on-findings")
	if code != ExitFindings {
		t.Fatalf("exit = %d, queria %d", code, ExitFindings)
	}
	if strings.Contains(out, "supersecreta") {
		t.Error("o valor do segredo vazou na saída")
	}
}
