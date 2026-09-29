// Package cli monta a árvore de comandos Cobra do forja.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/chwiee/forja/internal/build"
	"github.com/chwiee/forja/internal/registry"
)

// Exit codes: o CI decide o que fazer olhando para eles.
const (
	ExitOK       = 0
	ExitFailure  = 1 // erro inesperado (rede, parâmetro inválido, build quebrado...)
	ExitNotFound = 2 // exists: a tag não existe; push --immutable: a tag já existe
	ExitFindings = 3 // scan/run: reprovada no gate; inspect: achou variável sensível
)

// ExitError carrega um código de saída específico até o main.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

// Deps são as dependências externas. O main passa as reais; o teste, fakes.
type Deps struct {
	NewClient   func(registry.Config) registry.Client
	Engine      build.Engine
	Scanner     Scanner
	Credentials CredentialsFunc
}

// options guarda os valores das flags globais (persistent flags).
type options struct {
	timeout        time.Duration
	tlsVerify      bool
	output         string
	storageDriver  string
	registry       string
	registriesFile string

	deps   Deps
	client registry.Client
	log    io.Writer       // stderr do comando
	authed map[string]bool // hosts cuja credencial já foi instalada
}

// NewRootCmd monta o comando raiz e pendura os subcomandos nele.
func NewRootCmd(deps Deps) *cobra.Command {
	opts := &options{deps: deps}

	root := &cobra.Command{
		Use:   "forja",
		Short: "Builda, escaneia e publica imagens de container sem Docker",
		Long: `forja builda imagens a partir de um Dockerfile usando a biblioteca do buildah,
dentro do próprio processo: não precisa de Docker, daemon nem do binário buildah.

Exit codes: 0 ok | 1 erro | 2 tag inexistente (ou já existente com --immutable) | 3 reprovada no gate`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if opts.output != "table" && opts.output != "json" {
				return fmt.Errorf("--output deve ser table ou json, recebi %q", opts.output)
			}
			opts.client = opts.deps.NewClient(registry.Config{TLSVerify: opts.tlsVerify})
			opts.log = cmd.ErrOrStderr()
			return nil
		},
	}

	pf := root.PersistentFlags()
	pf.DurationVar(&opts.timeout, "timeout", 30*time.Minute, "tempo máximo do comando")
	pf.BoolVar(&opts.tlsVerify, "tls-verify", true, "valida o certificado TLS do registry")
	pf.StringVarP(&opts.output, "output", "o", "table", "formato de saída: table|json")
	pf.StringVar(&opts.storageDriver, "storage-driver", envOr("STORAGE_DRIVER", "vfs"), "driver de storage: vfs|overlay (env STORAGE_DRIVER)")
	pf.StringVar(&opts.registry, "registry", "", "registry por nome (ecr, ghcr): usa --name e --tag para montar a imagem")
	pf.StringVar(&opts.registriesFile, "registries", os.Getenv("FORJA_REGISTRIES"), "arquivo de registries (padrão: o embutido; env FORJA_REGISTRIES)")

	root.AddCommand(
		newBuildCmd(opts),
		newPushCmd(opts),
		newManifestCmd(opts),
		newScanCmd(opts),
		newRunCmd(opts),
		newExistsCmd(opts),
		newInspectCmd(opts),
		newVersionCmd(),
	)
	return root
}

// ctx cria um contexto com timeout para cada comando.
func (o *options) ctx(cmd *cobra.Command) (context.Context, context.CancelFunc) {
	return context.WithTimeout(cmd.Context(), o.timeout)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Execute roda o CLI e traduz o erro em exit code.
func Execute(deps Deps) int {
	err := NewRootCmd(deps).Execute()
	if err == nil {
		return ExitOK
	}
	fmt.Fprintln(os.Stderr, "erro:", err)
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return ExitFailure
}
