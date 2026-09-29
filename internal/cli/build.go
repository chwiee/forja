package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chwiee/forja/internal/build"
)

// buildFlags são as flags de build, usadas por build e por run.
type buildFlags struct {
	file      string
	tag       string
	buildArgs []string
	platforms []string
}

func (b *buildFlags) register(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVarP(&b.file, "file", "f", "Dockerfile", "caminho do Dockerfile (relativo ao contexto)")
	f.StringVarP(&b.tag, "tag", "t", "", "nome da imagem: registry/repo:tag")
	f.StringArrayVar(&b.buildArgs, "build-arg", nil, "variável de build CHAVE=VALOR (pode repetir)")
	f.StringSliceVar(&b.platforms, "platform", nil, "plataformas, ex.: linux/amd64,linux/arm64 (gera um manifest list)")
	_ = cmd.MarkFlagRequired("tag")
}

// options valida as flags e monta o build.Options.
func (b *buildFlags) options(args []string, opts *options, log io.Writer) (build.Options, error) {
	contextDir := "."
	if len(args) == 1 {
		contextDir = args[0]
	}
	file := b.file
	// Dockerfile relativo é procurado dentro do contexto, como no docker build.
	if !filepath.IsAbs(file) {
		file = filepath.Join(contextDir, file)
	}
	parsedArgs, err := parseBuildArgs(b.buildArgs)
	if err != nil {
		return build.Options{}, err
	}
	parsedPlatforms, err := parsePlatforms(b.platforms)
	if err != nil {
		return build.Options{}, err
	}
	return build.Options{
		Containerfile: file,
		ContextDir:    contextDir,
		Image:         b.tag,
		BuildArgs:     parsedArgs,
		Platforms:     parsedPlatforms,
		StorageDriver: opts.storageDriver,
		TLSVerify:     opts.tlsVerify,
		Out:           log,
	}, nil
}

func newBuildCmd(opts *options) *cobra.Command {
	var (
		bf   buildFlags
		push bool
	)
	cmd := &cobra.Command{
		Use:   "build [CONTEXTO]",
		Short: "Builda uma imagem a partir de um Dockerfile (CONTEXTO padrão: .)",
		Example: `  forja build -t registry.local/app:1.0 .
  forja build -f docker/Dockerfile -t ghcr.io/org/app:1.0 --build-arg VERSION=1.0 --push .
  forja build --platform linux/amd64,linux/arm64 -t ghcr.io/org/app:1.0 --push .`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log := cmd.ErrOrStderr() // logs do build no stderr; o resultado no stdout
			o, err := bf.options(args, opts, log)
			if err != nil {
				return err
			}
			ctx, cancel := opts.ctx(cmd)
			defer cancel()

			id, err := opts.deps.Engine.Build(ctx, o)
			if err != nil {
				return err
			}
			result := map[string]string{"image": bf.tag, "id": id}
			if push {
				digest, err := opts.deps.Engine.Push(ctx, build.PushOptions{
					Image: bf.tag, StorageDriver: opts.storageDriver, TLSVerify: opts.tlsVerify, Out: log,
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
			fmt.Fprintf(out, "BUILD OK  %s  id=%s\n", bf.tag, short(id))
			if push {
				fmt.Fprintf(out, "PUSH OK   %s  %s\n", bf.tag, result["digest"])
			}
			return nil
		},
	}
	bf.register(cmd)
	cmd.Flags().BoolVar(&push, "push", false, "publica a imagem no registry depois do build (sem scan; prefira forja run)")
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

func short(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
