// Package registry isola o forja da biblioteca containers/image.
// O resto do código só conhece a interface Client, nunca a lib diretamente.
package registry

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/docker/distribution/registry/api/errcode"
	v2 "github.com/docker/distribution/registry/api/v2"
	"go.podman.io/image/v5/docker"
	"go.podman.io/image/v5/image"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/image/v5/types"
)

// ErrNotFound indica que a imagem/tag não existe no registry.
var ErrNotFound = errors.New("imagem não encontrada no registry")

// ImageInfo é o recorte da configuração da imagem que nos interessa.
type ImageInfo struct {
	Digest string            `json:"digest"`
	User   string            `json:"user"`
	Env    []string          `json:"env"`
	Labels map[string]string `json:"labels"`
}

// Client é o contrato que os comandos usam. Em teste, trocamos por um fake.
type Client interface {
	Digest(ctx context.Context, image string) (string, error)
	Inspect(ctx context.Context, image string) (*ImageInfo, error)
}

// Config são as opções vindas das flags do CLI.
type Config struct {
	TLSVerify bool
}

// Remote implementa Client falando com um registry de verdade.
type Remote struct {
	sys *types.SystemContext
}

// NewRemote monta o SystemContext (o "objeto de configuração" da lib).
func NewRemote(cfg Config) *Remote {
	return &Remote{sys: &types.SystemContext{
		// Rodando no Windows, a lib escolheria a variante windows de uma
		// imagem multi-arch. Imagens de CI são linux: fixamos o OS.
		OSChoice:                    "linux",
		ArchitectureChoice:          runtime.GOARCH,
		DockerInsecureSkipTLSVerify: types.NewOptionalBool(!cfg.TLSVerify),
	}}
}

// Digest devolve o digest do manifesto de "registry/repo:tag".
func (r *Remote) Digest(ctx context.Context, name string) (string, error) {
	ref, err := docker.ParseReference("//" + name)
	if err != nil {
		return "", fmt.Errorf("referência inválida %q: %w", name, err)
	}
	// docker.GetDigest faria um HEAD (mais barato), mas no 404 devolve um tipo
	// de erro não exportado: não dá para separar "não existe" de outras falhas.
	// O GET do manifesto devolve o código MANIFEST_UNKNOWN, que dá para testar.
	src, err := ref.NewImageSource(ctx, r.sys)
	if err != nil {
		return "", translate(err)
	}
	defer src.Close()

	raw, _, err := src.GetManifest(ctx, nil)
	if err != nil {
		return "", translate(err)
	}
	d, err := manifest.Digest(raw)
	if err != nil {
		return "", err
	}
	return d.String(), nil
}

// Inspect lê a configuração (ENV, USER, LABELS) sem baixar as camadas.
func (r *Remote) Inspect(ctx context.Context, name string) (*ImageInfo, error) {
	ref, err := docker.ParseReference("//" + name)
	if err != nil {
		return nil, fmt.Errorf("referência inválida %q: %w", name, err)
	}
	src, err := ref.NewImageSource(ctx, r.sys)
	if err != nil {
		return nil, translate(err)
	}
	defer src.Close()

	img, err := image.FromUnparsedImage(ctx, r.sys, image.UnparsedInstance(src, nil))
	if err != nil {
		return nil, translate(err)
	}
	cfg, err := img.OCIConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("lendo config da imagem: %w", err)
	}
	return &ImageInfo{
		Digest: img.ConfigInfo().Digest.String(),
		User:   cfg.Config.User,
		Env:    cfg.Config.Env,
		Labels: cfg.Config.Labels,
	}, nil
}

// translate converte erros da lib em erros do nosso domínio.
// Cada registry embrulha o erro de um jeito: Docker Hub e Quay devolvem
// errcode.Error; o GHCR devolve errcode.ErrorCode. Testamos os dois.
func translate(err error) error {
	var e errcode.Error
	var code errcode.ErrorCode
	if (errors.As(err, &e) && isNotFound(e.Code)) || (errors.As(err, &code) && isNotFound(code)) {
		return fmt.Errorf("%w: %v", ErrNotFound, err)
	}
	return err
}

// isNotFound: MANIFEST_UNKNOWN = a tag não existe; NAME_UNKNOWN = nem o
// repositório existe (é o que o registry local "distribution" responde).
func isNotFound(c errcode.ErrorCode) bool {
	return c == v2.ErrorCodeManifestUnknown || c == v2.ErrorCodeNameUnknown
}
