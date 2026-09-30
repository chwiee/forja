package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version é trocado no build: go build -ldflags "-X github.com/chwiee/forja/internal/cli.version=1.0.0"
var version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Mostra a versão do forja",
		Args:  cobra.NoArgs,
		// Sobrescreve o PersistentPreRunE do root: version não precisa de cliente.
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "forja", version)
		},
	}
}
