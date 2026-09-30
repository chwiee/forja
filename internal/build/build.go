// Package build isola o forja da biblioteca do buildah.
// O código real fica em build_linux.go; em outros sistemas, build_other.go.
package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrUnsupported é devolvido fora do Linux: o buildah precisa do kernel Linux.
var ErrUnsupported = errors.New("build e push só funcionam em Linux; no Windows ou macOS rode a imagem do forja com Docker (veja 'Rodando no Windows' no Manual da Forja)")

// Options descreve um build.
type Options struct {
	Containerfile string            // caminho do Dockerfile/Containerfile
	ContextDir    string            // diretório usado por COPY e ADD
	Image         string            // nome final: registry/repo:tag
	BuildArgs     map[string]string // --build-arg
	Platforms     []Platform        // vazio = arquitetura do nó; 1+ = gera manifest list
	StorageDriver string            // vfs (padrão) ou overlay
	TLSVerify     bool
	Out           io.Writer // logs do build
}

// PushOptions descreve um push de uma imagem (ou manifest list) do storage local.
type PushOptions struct {
	Image         string
	StorageDriver string
	TLSVerify     bool
	Out           io.Writer
}

// ManifestOptions junta imagens já publicadas (uma por arquitetura) num
// manifest list e publica o resultado.
type ManifestOptions struct {
	Name          string   // manifest list final: registry/repo:tag
	Images        []string // imagens de origem, uma por arquitetura
	StorageDriver string
	TLSVerify     bool
	Out           io.Writer
}

// ExportOptions exporta uma imagem do storage local para diretório(s) OCI,
// que é o que o scan lê: os mesmos bytes que o push vai publicar.
type ExportOptions struct {
	Image         string
	Dir           string // diretório base; um subdiretório por arquitetura se for manifest list
	StorageDriver string
}

// Exported é uma imagem exportada: a plataforma e o diretório OCI dela.
type Exported struct {
	Platform string
	Dir      string
}

// Platform é um par sistema/arquitetura, como linux/arm64.
type Platform struct {
	OS, Arch, Variant string
}

func (p Platform) String() string {
	s := p.OS + "/" + p.Arch
	if p.Variant != "" {
		s += "/" + p.Variant
	}
	return s
}

// ParsePlatform aceita "linux/amd64", "linux/arm64" ou "linux/arm/v7".
func ParsePlatform(s string) (Platform, error) {
	parts := strings.Split(strings.TrimSpace(s), "/")
	if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || parts[1] == "" {
		return Platform{}, fmt.Errorf("plataforma %q inválida: use os/arquitetura, ex.: linux/arm64", s)
	}
	p := Platform{OS: parts[0], Arch: parts[1]}
	if len(parts) == 3 {
		p.Variant = parts[2]
	}
	return p, nil
}

// Engine é o contrato que os comandos usam. Em teste, trocamos por um fake.
type Engine interface {
	Build(ctx context.Context, o Options) (imageID string, err error)
	Push(ctx context.Context, o PushOptions) (digest string, err error)
	Manifest(ctx context.Context, o ManifestOptions) (digest string, err error)
	Export(ctx context.Context, o ExportOptions) ([]Exported, error)
}

// Buildah é a implementação real (ou o stub, fora do Linux).
type Buildah struct{}
