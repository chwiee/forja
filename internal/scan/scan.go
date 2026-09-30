package scan

import (
	"context"
	"fmt"
	"time"

	"go.podman.io/image/v5/docker"
	"go.podman.io/image/v5/oci/layout"
	"go.podman.io/image/v5/types"
)

// Target é a imagem a escanear: um diretório OCI local (exportado do
// storage pelo forja) OU uma imagem remota num registry.
type Target struct {
	Name      string // nome para o relatório
	OCIDir    string // diretório OCI local; vazio se for remota
	Remote    string // registry/repo:tag; vazio se for local
	TLSVerify bool
}

// Options controla o scan.
type Options struct {
	DBDir    string // onde o banco de CVEs do Grype fica em cache
	SkipCVEs bool   // pula o Grype (útil sem internet)
}

// Scan roda os três scanners e devolve os achados crus, sem nota.
// A nota vem de Policy.Evaluate, separada para poder ser testada sem rede.
func Scan(ctx context.Context, t Target, o Options) (*Report, error) {
	ref, syftInput, err := t.resolve()
	if err != nil {
		return nil, err
	}
	sys := &types.SystemContext{DockerInsecureSkipTLSVerify: types.NewOptionalBool(!t.TLSVerify)}

	r := &Report{Image: t.Name, ScannedAt: time.Now().UTC()}
	found, err := scanConfigAndLayers(ctx, ref, sys)
	if err != nil {
		return nil, err
	}
	r.Findings = append(r.Findings, found...)

	if !o.SkipCVEs {
		cves, pkgs, err := scanCVEs(ctx, syftInput, o.DBDir, !t.TLSVerify)
		if err != nil {
			return nil, err
		}
		r.Findings = append(r.Findings, cves...)
		r.Packages = pkgs
	}
	return r, nil
}

// resolve traduz o alvo para as duas bibliotecas: a referência do
// containers/image (config e camadas) e a entrada do Syft (pacotes).
func (t Target) resolve() (types.ImageReference, string, error) {
	switch {
	case t.OCIDir != "":
		ref, err := layout.NewReference(t.OCIDir, "forja")
		return ref, "oci-dir:" + t.OCIDir, err
	case t.Remote != "":
		ref, err := docker.ParseReference("//" + t.Remote)
		return ref, "registry:" + t.Remote, err
	}
	return nil, "", fmt.Errorf("alvo de scan vazio")
}
