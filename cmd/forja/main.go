package main

import (
	"os"

	"github.com/chwiee/forja/internal/cli"
	"github.com/chwiee/forja/internal/registry"
)

func main() {
	newClient := func(cfg registry.Config) registry.Client { return registry.NewRemote(cfg) }
	os.Exit(cli.Execute(newClient))
}
