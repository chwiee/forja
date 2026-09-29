//go:build linux

package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"go.podman.io/buildah"
	"go.podman.io/buildah/define"
	"go.podman.io/buildah/imagebuildah"
	"go.podman.io/common/libimage"
	imagecopy "go.podman.io/image/v5/copy"
	"go.podman.io/image/v5/docker"
	"go.podman.io/image/v5/oci/layout"
	"go.podman.io/image/v5/types"
	"go.podman.io/storage"
)

// InitReexec precisa ser a primeira coisa do main: o buildah re-executa o
// próprio binário para rodar etapas isoladas, e é aqui que o filho desvia.
func InitReexec() bool { return buildah.InitReexec() }

// openStore abre o storage local de imagens (/var/lib/containers/storage).
func openStore(driver string) (storage.Store, error) {
	opts, err := storage.DefaultStoreOptions()
	if err != nil {
		return nil, fmt.Errorf("lendo configuração de storage: %w", err)
	}
	if driver != "" {
		opts.GraphDriverName = driver
		opts.GraphDriverOptions = nil
	}
	store, err := storage.GetStore(opts)
	if err != nil {
		return nil, fmt.Errorf("abrindo storage %q: %w", driver, err)
	}
	return store, nil
}

// systemContext monta a configuração de acesso a registries: TLS e os
// arquivos de política (do sistema ou os embutidos no binário).
func systemContext(tlsVerify bool) (*types.SystemContext, error) {
	policy, registries, err := configPaths()
	if err != nil {
		return nil, err
	}
	return &types.SystemContext{
		DockerInsecureSkipTLSVerify: types.NewOptionalBool(!tlsVerify),
		SignaturePolicyPath:         policy,
		SystemRegistriesConfPath:    registries,
	}, nil
}

// Build executa o Containerfile com a biblioteca do buildah, no próprio processo.
// Com Platforms preenchido, builda uma imagem por plataforma e junta todas num
// manifest list chamado o.Image, pronto para o Push.
func (Buildah) Build(ctx context.Context, o Options) (string, error) {
	sys, err := systemContext(o.TLSVerify)
	if err != nil {
		return "", err
	}
	store, err := openStore(o.StorageDriver)
	if err != nil {
		return "", err
	}
	defer func() { _, _ = store.Shutdown(false) }()

	opts := define.BuildOptions{
		ContextDirectory: o.ContextDir,
		Output:           o.Image,
		Args:             o.BuildArgs,
		// chroot: isolamento em Go puro. Não precisa de runc/crun na imagem.
		Isolation: define.IsolationChroot,
		// RUN usa a rede do container do forja. Sem isso, o buildah procura o
		// netavark para criar uma rede isolada, e ele não existe no distroless.
		NamespaceOptions:    []define.NamespaceOption{{Name: string(specs.NetworkNamespace), Host: true}},
		ConfigureNetwork:    define.NetworkEnabled,
		NetworkInterface:    hostNetwork{},
		PullPolicy:          define.PullIfMissing,
		OutputFormat:        define.OCIv1ImageManifest,
		SystemContext:       sys,
		SignaturePolicyPath: sys.SignaturePolicyPath,
		Out:                 o.Out,
		Err:                 o.Out,
		ReportWriter:        o.Out,
	}
	if len(o.Platforms) > 0 {
		// Multi-arch: cada imagem sai sem nome e entra no manifest list o.Image.
		opts.Output = ""
		opts.Manifest = o.Image
		for _, p := range o.Platforms {
			opts.Platforms = append(opts.Platforms, struct{ OS, Arch, Variant string }{p.OS, p.Arch, p.Variant})
		}
	}

	id, _, err := imagebuildah.BuildDockerfiles(ctx, store, opts, o.Containerfile)
	if err != nil {
		return "", fmt.Errorf("build: %w", err)
	}
	return id, nil
}

// Push publica uma imagem do storage local no registry. Se o nome for de um
// manifest list, publica o list e todas as imagens dele.
func (Buildah) Push(ctx context.Context, o PushOptions) (string, error) {
	sys, err := systemContext(o.TLSVerify)
	if err != nil {
		return "", err
	}
	store, err := openStore(o.StorageDriver)
	if err != nil {
		return "", err
	}
	defer func() { _, _ = store.Shutdown(false) }()

	rt, err := libimage.RuntimeFromStore(store, &libimage.RuntimeOptions{SystemContext: sys})
	if err != nil {
		return "", err
	}
	list, err := rt.LookupManifestList(o.Image)
	switch {
	case err == nil:
		return pushList(ctx, list, o.Image, sys, o.Out)
	case !errors.Is(err, storage.ErrImageUnknown) && !errors.Is(err, libimage.ErrNotAManifestList):
		return "", err
	}

	dest, err := docker.ParseReference("//" + o.Image)
	if err != nil {
		return "", fmt.Errorf("referência inválida %q: %w", o.Image, err)
	}
	_, digest, err := buildah.Push(ctx, o.Image, dest, buildah.PushOptions{
		Store:               store,
		SystemContext:       sys,
		SignaturePolicyPath: sys.SignaturePolicyPath,
		ReportWriter:        o.Out,
	})
	if err != nil {
		return "", fmt.Errorf("push: %w", err)
	}
	return digest.String(), nil
}

// Manifest junta imagens já publicadas num manifest list e publica o list.
// É o caminho do EKS com nós mistos: cada arquitetura builda no próprio nó
// (sem emulação) e um passo final só junta os manifestos no registry.
func (Buildah) Manifest(ctx context.Context, o ManifestOptions) (string, error) {
	sys, err := systemContext(o.TLSVerify)
	if err != nil {
		return "", err
	}
	store, err := openStore(o.StorageDriver)
	if err != nil {
		return "", err
	}
	defer func() { _, _ = store.Shutdown(false) }()

	rt, err := libimage.RuntimeFromStore(store, &libimage.RuntimeOptions{SystemContext: sys})
	if err != nil {
		return "", err
	}
	// Recomeça do zero se já existir um list local com esse nome.
	if old, err := rt.LookupManifestList(o.Name); err == nil {
		if _, err := store.DeleteImage(old.ID(), true); err != nil {
			return "", fmt.Errorf("removendo manifest list antigo: %w", err)
		}
	}
	list, err := rt.CreateManifestList(o.Name)
	if err != nil {
		return "", fmt.Errorf("criando manifest list: %w", err)
	}
	for _, img := range o.Images {
		if _, err := list.Add(ctx, "docker://"+img, &libimage.ManifestListAddOptions{
			InsecureSkipTLSVerify: types.NewOptionalBool(!o.TLSVerify),
		}); err != nil {
			return "", fmt.Errorf("adicionando %s: %w", img, err)
		}
	}
	return pushList(ctx, list, o.Name, sys, o.Out)
}

// pushList publica o manifest list e todas as imagens que ele referencia.
func pushList(ctx context.Context, list *libimage.ManifestList, name string, sys *types.SystemContext, out io.Writer) (string, error) {
	opts := &libimage.ManifestListPushOptions{ImageListSelection: imagecopy.CopyAllImages}
	opts.SignaturePolicyPath = sys.SignaturePolicyPath
	opts.InsecureSkipTLSVerify = sys.DockerInsecureSkipTLSVerify
	opts.Writer = out
	digest, err := list.Push(ctx, "docker://"+name, opts)
	if err != nil {
		return "", fmt.Errorf("push do manifest list: %w", err)
	}
	return digest.String(), nil
}

// Export grava a imagem (ou cada arquitetura de um manifest list) num
// diretório OCI. O scan lê esse diretório: são os bytes que o push publica.
func (Buildah) Export(ctx context.Context, o ExportOptions) ([]Exported, error) {
	sys, err := systemContext(true)
	if err != nil {
		return nil, err
	}
	store, err := openStore(o.StorageDriver)
	if err != nil {
		return nil, err
	}
	defer func() { _, _ = store.Shutdown(false) }()

	rt, err := libimage.RuntimeFromStore(store, &libimage.RuntimeOptions{SystemContext: sys})
	if err != nil {
		return nil, err
	}

	list, err := rt.LookupManifestList(o.Image)
	if err != nil {
		if !errors.Is(err, storage.ErrImageUnknown) && !errors.Is(err, libimage.ErrNotAManifestList) {
			return nil, err
		}
		// imagem simples
		if err := exportOne(ctx, store, sys, o.Image, o.Dir); err != nil {
			return nil, err
		}
		return []Exported{{Platform: "", Dir: o.Dir}}, nil
	}

	data, err := list.Inspect()
	if err != nil {
		return nil, fmt.Errorf("lendo manifest list: %w", err)
	}
	var out []Exported
	for _, m := range data.Manifests {
		p := Platform{OS: m.Platform.OS, Arch: m.Platform.Architecture, Variant: m.Platform.Variant}
		img, err := list.LookupInstance(ctx, p.Arch, p.OS, p.Variant)
		if err != nil {
			return nil, fmt.Errorf("achando a imagem %s do manifest list: %w", p, err)
		}
		dir := filepath.Join(o.Dir, strings.ReplaceAll(p.String(), "/", "-"))
		if err := exportOne(ctx, store, sys, img.ID(), dir); err != nil {
			return nil, err
		}
		out = append(out, Exported{Platform: p.String(), Dir: dir})
	}
	return out, nil
}

func exportOne(ctx context.Context, store storage.Store, sys *types.SystemContext, image, dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	dest, err := layout.NewReference(dir, "forja")
	if err != nil {
		return err
	}
	if _, _, err := buildah.Push(ctx, image, dest, buildah.PushOptions{
		Store:               store,
		SystemContext:       sys,
		SignaturePolicyPath: sys.SignaturePolicyPath,
		ReportWriter:        io.Discard,
	}); err != nil {
		return fmt.Errorf("exportando %s para %s: %w", image, dir, err)
	}
	return nil
}
