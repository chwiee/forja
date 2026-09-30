// Package cli monta a árvore de comandos Cobra.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/chwiee/forja/internal/registry"
)

// Exit codes: o CI decide o que fazer olhando para eles.
const (
	ExitOK       = 0
	ExitFailure  = 1 // erro inesperado (rede, parâmetro inválido...)
	ExitNotFound = 2 // exists: a tag não existe
	ExitFindings = 3 // inspect: achou variável sensível hardcoded
)

// ExitError carrega um código de saída específico até o main.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

// ClientFactory cria o cliente depois que as flags foram lidas.
type ClientFactory func(registry.Config) registry.Client

// options guarda os valores das flags globais (persistent flags).
type options struct {
	timeout   time.Duration
	tlsVerify bool
	output    string

	newClient ClientFactory
	client    registry.Client
}

// NewRootCmd monta o comando raiz e pendura os subcomandos nele.
func NewRootCmd(newClient ClientFactory) *cobra.Command {
	opts := &options{newClient: newClient}

	root := &cobra.Command{
		Use:   "forja",
		Short: "Consulta imagens de container direto no registry",
		Long: `forja é o CLI de exemplo do ebook "Forja: do zero ao CLI de CI".
Ele verifica se uma tag existe e inspeciona a configuração de uma imagem
sem precisar de Docker instalado.`,
		SilenceUsage:  true, // não imprime o --help inteiro a cada erro
		SilenceErrors: true, // quem imprime o erro é o Execute, uma vez só
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if opts.output != "table" && opts.output != "json" {
				return fmt.Errorf("--output deve ser table ou json, recebi %q", opts.output)
			}
			opts.client = opts.newClient(registry.Config{TLSVerify: opts.tlsVerify})
			return nil
		},
	}

	pf := root.PersistentFlags()
	pf.DurationVar(&opts.timeout, "timeout", 30*time.Second, "tempo máximo de cada chamada ao registry")
	pf.BoolVar(&opts.tlsVerify, "tls-verify", true, "valida o certificado TLS do registry")
	pf.StringVarP(&opts.output, "output", "o", "table", "formato de saída: table|json")

	root.AddCommand(newExistsCmd(opts), newInspectCmd(opts), newVersionCmd())
	return root
}

// ctx cria um contexto com timeout para cada comando.
func (o *options) ctx(cmd *cobra.Command) (context.Context, context.CancelFunc) {
	return context.WithTimeout(cmd.Context(), o.timeout)
}

// Execute roda o CLI e traduz o erro em exit code.
func Execute(newClient ClientFactory) int {
	err := NewRootCmd(newClient).Execute()
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
