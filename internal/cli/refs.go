package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chwiee/forja/internal/registries"
)

// CredentialsFunc obtém a credencial de um registry (no main: SDK da AWS).
type CredentialsFunc func(ctx context.Context, r registries.Registry) (*registries.Credentials, error)

// imageArg são as flags para montar a imagem sem digitar o endereço:
// forja push --registry ecr --name org/app --tag v1.0.0
type imageArg struct {
	name string
	tag  string
}

func (a *imageArg) register(cmd *cobra.Command, withTag bool) {
	cmd.Flags().StringVar(&a.name, "name", "", "repositório no registry (ex.: ${{ github.repository }}); exige --registry")
	if withTag {
		cmd.Flags().StringVar(&a.tag, "tag", "", "tag (ex.: ${{ github.ref }}); exige --registry")
	}
}

// imageFromArgs resolve a imagem de comandos que recebem IMAGEM posicional.
func (o *options) imageFromArgs(ctx context.Context, args []string, a imageArg) (string, error) {
	positional := ""
	if len(args) > 0 {
		positional = args[0]
	}
	if o.registry != "" && positional != "" {
		return "", errors.New("com --registry, use --name e --tag em vez do endereço completo")
	}
	if o.registry == "" {
		if a.name != "" || a.tag != "" {
			return "", errors.New("--name e --tag exigem --registry (ecr, ghcr...)")
		}
		if positional == "" {
			return "", errors.New("informe a IMAGEM, ou use --registry, --name e --tag")
		}
		return o.withAuth(ctx, positional)
	}
	return o.resolve(ctx, a.name, a.tag)
}

// resolve monta a imagem: com --registry, nameOrEmpty + tag viram
// host/nome:tag normalizados; sem --registry, tagOrRef já é o endereço.
func (o *options) resolve(ctx context.Context, name, tagOrRef string) (string, error) {
	if o.registry == "" {
		if name != "" {
			return "", errors.New("--name exige --registry (ecr, ghcr...)")
		}
		return o.withAuth(ctx, tagOrRef)
	}
	if name == "" {
		return "", errors.New("com --registry, informe --name (ex.: --name ${{ github.repository }})")
	}
	reg, err := o.lookupRegistry()
	if err != nil {
		return "", err
	}
	ref, err := reg.Reference(name, tagOrRef)
	if err != nil {
		return "", err
	}
	return o.withAuth(ctx, ref)
}

func (o *options) lookupRegistry() (registries.Registry, error) {
	regs, err := registries.Load(o.registriesFile)
	if err != nil {
		return registries.Registry{}, err
	}
	return registries.Lookup(regs, o.registry)
}

// withAuth garante a credencial quando o destino é um ECR, seja via
// --registry ecr ou por um endereço completo *.dkr.ecr.*.amazonaws.com.
func (o *options) withAuth(ctx context.Context, ref string) (string, error) {
	host, _, _ := strings.Cut(ref, "/")
	var reg registries.Registry
	if o.registry != "" {
		r, err := o.lookupRegistry()
		if err != nil {
			return "", err
		}
		if r.Host == host {
			reg = r
		}
	}
	if reg.Type == "" {
		if r, ok := registries.DetectECR(host); ok {
			reg = r
		}
	}
	if reg.Type != registries.TypeECR || o.authed[host] {
		return ref, nil
	}
	creds, err := o.deps.Credentials(ctx, reg)
	if err != nil {
		return "", err
	}
	if _, err := registries.InstallAuth(filepath.Join(os.TempDir(), "forja-auth"), host, creds); err != nil {
		return "", err
	}
	if o.authed == nil {
		o.authed = map[string]bool{}
	}
	o.authed[host] = true
	fmt.Fprintf(o.log, "credencial do ECR obtida para %s\n", host)
	return ref, nil
}
