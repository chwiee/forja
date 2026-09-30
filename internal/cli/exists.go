package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chwiee/forja/internal/registry"
)

func newExistsCmd(opts *options) *cobra.Command {
	var img imageArg
	cmd := &cobra.Command{
		Use:   "exists [IMAGEM]",
		Short: "Verifica se registry/repo:tag existe (exit 0 = existe, 2 = não existe)",
		Example: `  forja exists docker.io/library/alpine:3.20
  forja exists localhost:5000/app:1.0 --tls-verify=false -o json
  forja exists --registry ecr --name org/app --tag v1.0.0`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := opts.ctx(cmd)
			defer cancel()
			image, err := opts.imageFromArgs(ctx, args, img)
			if err != nil {
				return err
			}

			digest, err := opts.client.Digest(ctx, image)
			found := err == nil
			if err != nil && !errors.Is(err, registry.ErrNotFound) {
				return err // erro de verdade: rede, auth, nome inválido
			}

			out := cmd.OutOrStdout()
			if opts.output == "json" {
				_ = json.NewEncoder(out).Encode(map[string]any{
					"image": image, "exists": found, "digest": digest,
				})
			} else if found {
				fmt.Fprintf(out, "EXISTE  %s  %s\n", image, digest)
			} else {
				fmt.Fprintf(out, "NÃO EXISTE  %s\n", image)
			}

			if !found {
				return &ExitError{Code: ExitNotFound, Err: fmt.Errorf("%s não existe", image)}
			}
			return nil
		},
	}
	img.register(cmd, true)
	return cmd
}
