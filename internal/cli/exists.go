package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chwiee/forja/internal/registry"
)

func newExistsCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "exists IMAGEM",
		Short: "Verifica se registry/repo:tag existe (exit 0 = existe, 2 = não existe)",
		Example: `  forja exists docker.io/library/alpine:3.20
  forja exists localhost:5000/app:1.0 --tls-verify=false -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := opts.ctx(cmd)
			defer cancel()

			digest, err := opts.client.Digest(ctx, args[0])
			found := err == nil
			if err != nil && !errors.Is(err, registry.ErrNotFound) {
				return err // erro de verdade: rede, auth, nome inválido
			}

			out := cmd.OutOrStdout()
			if opts.output == "json" {
				_ = json.NewEncoder(out).Encode(map[string]any{
					"image": args[0], "exists": found, "digest": digest,
				})
			} else if found {
				fmt.Fprintf(out, "EXISTE  %s  %s\n", args[0], digest)
			} else {
				fmt.Fprintf(out, "NÃO EXISTE  %s\n", args[0])
			}

			if !found {
				return &ExitError{Code: ExitNotFound, Err: fmt.Errorf("%s não existe", args[0])}
			}
			return nil
		},
	}
}
