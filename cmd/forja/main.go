package main

import (
	"os"

	// Certificados CA embutidos, usados só se a imagem não tiver nenhum
	// (ex.: debian-slim, scratch). Com certificados no sistema, eles vencem.
	_ "golang.org/x/crypto/x509roots/fallback"

	"github.com/chwiee/forja/internal/build"
	"github.com/chwiee/forja/internal/cli"
	"github.com/chwiee/forja/internal/registry"
)

func main() {
	// Tem que ser a primeira linha: o buildah re-executa este binário
	// para rodar etapas isoladas, e o processo filho desvia aqui.
	if build.InitReexec() {
		return
	}
	os.Exit(cli.Execute(cli.Deps{
		NewClient: func(cfg registry.Config) registry.Client { return registry.NewRemote(cfg) },
		Engine:    build.Buildah{},
	}))
}
