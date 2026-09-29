package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chwiee/forja/internal/build"
)

func newManifestCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "manifest LISTA IMAGEM [IMAGEM...]",
		Short: "Junta imagens já publicadas (uma por arquitetura) num manifest list e publica",
		Long: `Junta imagens de arquiteturas diferentes num único nome.

Uso típico no EKS com nós mistos: cada arquitetura é buildada no próprio nó,
sem emulação, e publicada com um sufixo; este comando junta todas:

  (nó amd64)  forja build -t reg/app:1.0-amd64 --push .
  (nó arm64)  forja build -t reg/app:1.0-arm64 --push .
  (qualquer)  forja manifest reg/app:1.0 reg/app:1.0-amd64 reg/app:1.0-arm64

Depois disso, cada nó que puxar reg/app:1.0 recebe a imagem da própria arquitetura.`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := opts.ctx(cmd)
			defer cancel()

			digest, err := opts.deps.Engine.Manifest(ctx, build.ManifestOptions{
				Name:          args[0],
				Images:        args[1:],
				StorageDriver: opts.storageDriver,
				TLSVerify:     opts.tlsVerify,
				Out:           cmd.ErrOrStderr(),
			})
			if err != nil {
				return err
			}
			if opts.output == "json" {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"manifest": args[0], "images": args[1:], "digest": digest,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "MANIFEST OK  %s  %s  (%d imagens)\n", args[0], digest, len(args)-1)
			return nil
		},
	}
}
