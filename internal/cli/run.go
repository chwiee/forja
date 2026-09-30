package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chwiee/forja/internal/build"
	"github.com/chwiee/forja/internal/registry"
)

func newRunCmd(opts *options) *cobra.Command {
	var (
		bf        buildFlags
		sf        scanFlags
		immutable bool
	)
	cmd := &cobra.Command{
		Use:   "run [CONTEXTO]",
		Short: "Pipeline completo: exists → build → scan → gate → push",
		Long: `Roda a esteira inteira e para no primeiro problema:

  1. exists  com --immutable, falha (exit 2) se a tag já existe no registry
  2. build   builda o Dockerfile (uma ou várias arquiteturas)
  3. scan    CVEs, secrets e configuração, com nota segundo a política
  4. gate    reprovada: exit 3 e NADA é publicado
  5. push    aprovada: publica e imprime o digest`,
		Example: `  forja run -t ghcr.io/org/app:1.0 --immutable --sarif forja.sarif .
  forja run -t ghcr.io/org/app:1.0 --platform linux/amd64,linux/arm64 --policy forja-policy.yaml .
  forja run --registry ecr --name ${{ github.repository }} -t ${{ github.ref }} --immutable .`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := opts.ctx(cmd)
			defer cancel()
			log := cmd.ErrOrStderr()

			// resolve a imagem (e a credencial do ECR) antes de tudo
			o, err := bf.options(ctx, args, opts, log)
			if err != nil {
				return err
			}
			image := o.Image

			// 1. exists
			if immutable {
				_, err := opts.client.Digest(ctx, image)
				switch {
				case err == nil:
					return &ExitError{Code: ExitNotFound, Err: fmt.Errorf("%s já existe no registry e --immutable está ligado", image)}
				case !errors.Is(err, registry.ErrNotFound):
					return err
				}
				fmt.Fprintf(log, "[1/5] exists: %s ainda não existe\n", image)
			}

			// 2. build
			fmt.Fprintln(log, "[2/5] build")
			if _, err := opts.deps.Engine.Build(ctx, o); err != nil {
				return err
			}

			// 3 e 4. scan + gate (runScan devolve ExitError 3 se reprovar)
			fmt.Fprintln(log, "[3/5] scan")
			if err := runScan(ctx, cmd, opts, sf, image, false); err != nil {
				var exitErr *ExitError
				if errors.As(err, &exitErr) {
					fmt.Fprintln(log, "[4/5] gate: REPROVADA, nada foi publicado")
				}
				return err
			}
			fmt.Fprintln(log, "[4/5] gate: aprovada")

			// 5. push
			fmt.Fprintln(log, "[5/5] push")
			digest, err := opts.deps.Engine.Push(ctx, build.PushOptions{
				Image: image, StorageDriver: opts.storageDriver, TLSVerify: opts.tlsVerify, Out: log,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "PUSH OK   %s  %s\n", image, digest)
			return nil
		},
	}
	bf.register(cmd)
	sf.register(cmd)
	cmd.Flags().BoolVar(&immutable, "immutable", false, "falha (exit 2) se a tag já existir no registry")
	return cmd
}
