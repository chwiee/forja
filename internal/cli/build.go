package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chwiee/forja/internal/build"
)

func newBuildCmd(opts *options) *cobra.Command {
	var (
		file      string
		tag       string
		buildArgs []string
		platforms []string
		push      bool
	)

	cmd := &cobra.Command{
		Use:   "build [CONTEXTO]",
		Short: "Builda uma imagem a partir de um Dockerfile (CONTEXTO padrão: .)",
		Example: `  forja build -t registry.local/app:1.0 .
  forja build -f docker/Dockerfile -t ghcr.io/org/app:1.0 --build-arg VERSION=1.0 --push .
  forja build --platform linux/amd64,linux/arm64 -t ghcr.io/org/app:1.0 --push .`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contextDir := "."
			if len(args) == 1 {
				contextDir = args[0]
			}
			// Dockerfile relativo é procurado dentro do contexto, como no docker build.
			if !filepath.IsAbs(file) {
				file = filepath.Join(contextDir, file)
			}
			parsedArgs, err := parseBuildArgs(buildArgs)
			if err != nil {
				return err
			}
			parsedPlatforms, err := parsePlatforms(platforms)
			if err != nil {
				return err
			}

			ctx, cancel := opts.ctx(cmd)
			defer cancel()

			log := cmd.ErrOrStderr() // logs do build no stderr; o resultado no stdout
			id, err := opts.deps.Engine.Build(ctx, build.Options{
				Containerfile: file,
				ContextDir:    contextDir,
				Image:         tag,
				BuildArgs:     parsedArgs,
				Platforms:     parsedPlatforms,
				StorageDriver: opts.storageDriver,
				TLSVerify:     opts.tlsVerify,
				Out:           log,
			})
			if err != nil {
				return err
			}

			result := map[string]string{"image": tag, "id": id}
			if push {
				digest, err := opts.deps.Engine.Push(ctx, build.PushOptions{
					Image: tag, StorageDriver: opts.storageDriver, TLSVerify: opts.tlsVerify, Out: log,
				})
				if err != nil {
					return err
				}
				result["digest"] = digest
			}

			out := cmd.OutOrStdout()
			if opts.output == "json" {
				return json.NewEncoder(out).Encode(result)
			}
			fmt.Fprintf(out, "BUILD OK  %s  id=%s\n", tag, short(id))
			if push {
				fmt.Fprintf(out, "PUSH OK   %s  %s\n", tag, result["digest"])
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&file, "file", "f", "Dockerfile", "caminho do Dockerfile (relativo ao contexto)")
	f.StringVarP(&tag, "tag", "t", "", "nome da imagem: registry/repo:tag")
	f.StringArrayVar(&buildArgs, "build-arg", nil, "variável de build CHAVE=VALOR (pode repetir)")
	f.StringSliceVar(&platforms, "platform", nil, "plataformas, ex.: linux/amd64,linux/arm64 (gera um manifest list)")
	f.BoolVar(&push, "push", false, "publica a imagem no registry depois do build")
	_ = cmd.MarkFlagRequired("tag")
	return cmd
}

func parseBuildArgs(in []string) (map[string]string, error) {
	out := make(map[string]string, len(in))
	for _, kv := range in {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("--build-arg %q: use o formato CHAVE=VALOR", kv)
		}
		out[k] = v
	}
	return out, nil
}

func short(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func parsePlatforms(in []string) ([]build.Platform, error) {
	out := make([]build.Platform, 0, len(in))
	for _, s := range in {
		p, err := build.ParsePlatform(s)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}
