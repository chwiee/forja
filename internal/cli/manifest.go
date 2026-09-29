package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chwiee/forja/internal/build"
)

func newManifestCmd(opts *options) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "manifest LISTA IMAGEM [IMAGEM...]",
		Short: "Junta imagens já publicadas (uma por arquitetura) num manifest list e publica",
		Long: `Junta imagens de arquiteturas diferentes num único nome.

Uso típico no EKS com nós mistos: cada arquitetura é buildada no próprio nó,
sem emulação, e publicada com um sufixo; este comando junta todas:

  (nó amd64)  forja build -t reg/app:1.0-amd64 --push .
  (nó arm64)  forja build -t reg/app:1.0-arm64 --push .
  (qualquer)  forja manifest reg/app:1.0 reg/app:1.0-amd64 reg/app:1.0-arm64

Com --registry e --name, os argumentos são só as tags:

  forja manifest --registry ecr --name org/app v1.0 v1.0-amd64 v1.0-arm64

Depois disso, cada nó que puxar reg/app:1.0 recebe a imagem da própria arquitetura.`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := opts.ctx(cmd)
			defer cancel()

			refs := make([]string, len(args))
			for i, a := range args {
				if opts.registry != "" {
					r, err := opts.resolve(ctx, name, a) // a é só a tag
					if err != nil {
						return err
					}
					refs[i] = r
					continue
				}
				if name != "" {
					return fmt.Errorf("--name exige --registry")
				}
				r, err := opts.withAuth(ctx, a)
				if err != nil {
					return err
				}
				refs[i] = r
			}

			digest, err := opts.deps.Engine.Manifest(ctx, build.ManifestOptions{
				Name:          refs[0],
				Images:        refs[1:],
				StorageDriver: opts.storageDriver,
				TLSVerify:     opts.tlsVerify,
				Out:           cmd.ErrOrStderr(),
			})
			if err != nil {
				return err
			}
			if opts.output == "json" {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"manifest": refs[0], "images": refs[1:], "digest": digest,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "MANIFEST OK  %s  %s  (%d imagens)\n", refs[0], digest, len(refs)-1)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "repositório no registry; com --registry, os argumentos viram tags")
	return cmd
}
