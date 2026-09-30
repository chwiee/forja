//go:build linux

package build

import (
	"context"
	"errors"
	"fmt"
	"io"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"go.podman.io/buildah"
	"go.podman.io/buildah/define"
	"go.podman.io/buildah/imagebuildah"
	"go.podman.io/common/libimage"
	imagecopy "go.podman.io/image/v5/copy"
	"go.podman.io/image/v5/docker"
	"go.podman.io/image/v5/types"
	"go.podman.io/storage"
	"golang.org/x/sys/unix"
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
		NamespaceOptions: []define.NamespaceOption{{Name: string(specs.NetworkNamespace), Host: true}},
		ConfigureNetwork: define.NetworkEnabled,
		NetworkInterface: hostNetwork{},
		// Sem isto, o buildah dá a cada RUN nofile/nproc = 1048576, e subir
		// o limite acima do teto atual exige CAP_SYS_RESOURCE. Os runners do
		// GitHub têm teto de 65536: o RUN falhava com "operation not permitted".
		CommonBuildOpts:     &define.CommonBuildOptions{Ulimit: currentUlimits()},
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

// currentUlimits devolve os tetos atuais do processo para nofile e nproc,
// no formato do buildah ("nofile=65536:65536"). Usar o teto atual nunca
// exige privilégio extra; um valor maior exigiria CAP_SYS_RESOURCE.
func currentUlimits() []string {
	var out []string
	for name, res := range map[string]int{"nofile": unix.RLIMIT_NOFILE, "nproc": unix.RLIMIT_NPROC} {
		var r unix.Rlimit
		if err := unix.Getrlimit(res, &r); err == nil && r.Max != unix.RLIM_INFINITY {
			out = append(out, fmt.Sprintf("%s=%d:%d", name, r.Max, r.Max))
		}
	}
	return out
}
