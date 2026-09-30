package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chwiee/forja/internal/build"
	"github.com/chwiee/forja/internal/registry"
)

func newPushCmd(opts *options) *cobra.Command {
	var immutable bool

	cmd := &cobra.Command{
		Use:     "push IMAGEM",
		Short:   "Publica no registry uma imagem buildada antes (mesmo storage)",
		Example: "  forja push registry.local/app:1.0 --immutable",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			image := args[0]
			ctx, cancel := opts.ctx(cmd)
			defer cancel()

			if immutable {
				_, err := opts.client.Digest(ctx, image)
				switch {
				case err == nil:
					return &ExitError{Code: ExitNotFound, Err: fmt.Errorf("%s já existe no registry e --immutable está ligado", image)}
				case !errors.Is(err, registry.ErrNotFound):
					return err
				}
			}

			digest, err := opts.deps.Engine.Push(ctx, build.PushOptions{
				Image: image, StorageDriver: opts.storageDriver, TLSVerify: opts.tlsVerify, Out: cmd.ErrOrStderr(),
			})
			if err != nil {
				return err
			}
			if opts.output == "json" {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"image": image, "digest": digest})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "PUSH OK  %s  %s\n", image, digest)
			return nil
		},
	}
	cmd.Flags().BoolVar(&immutable, "immutable", false, "falha (exit 2) se a tag já existir no registry")
	return cmd
}
