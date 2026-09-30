package scan

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"github.com/zricethezav/gitleaks/v8/detect"
	"go.podman.io/image/v5/image"
	"go.podman.io/image/v5/pkg/blobinfocache/none"
	"go.podman.io/image/v5/pkg/compression"
	"go.podman.io/image/v5/types"
)

// SensitiveName pega nomes de variáveis que costumam guardar segredo.
var SensitiveName = regexp.MustCompile(`(?i)(pass(word)?|secret|token|api[_-]?key|private[_-]?key|credential)`)

// buildArgInHistory pega "--build-arg" gravado no histórico: "|2 A=1 B=2 /bin/sh -c ...".
var buildArgInHistory = regexp.MustCompile(`^\|\d+ ((?:[A-Za-z_][A-Za-z0-9_]*=\S*\s)+)`)

const maxFileSize = 2 << 20 // arquivos maiores que 2 MiB não são lidos

// Diretórios que só têm documentação e exemplos: geram falso positivo.
var skipDirs = []string{"usr/share/doc/", "usr/share/man/", "usr/share/licenses/"}

// scanConfigAndLayers lê a configuração e TODAS as camadas da imagem.
// Um segredo apagado numa camada de cima continua na camada de baixo, e é
// baixado por quem puxa a imagem: por isso cada camada é lida por inteiro.
func scanConfigAndLayers(ctx context.Context, ref types.ImageReference, sys *types.SystemContext) ([]Finding, error) {
	src, err := ref.NewImageSource(ctx, sys)
	if err != nil {
		return nil, fmt.Errorf("abrindo imagem: %w", err)
	}
	defer src.Close()
	img, err := image.FromUnparsedImage(ctx, sys, image.UnparsedInstance(src, nil))
	if err != nil {
		return nil, fmt.Errorf("lendo imagem: %w", err)
	}
	cfg, err := img.OCIConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("lendo config: %w", err)
	}

	detector, err := detect.NewDetectorDefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("iniciando gitleaks: %w", err)
	}

	var out []Finding
	// 1. Usuário
	if u := cfg.Config.User; u == "" || u == "root" || u == "0" || strings.HasPrefix(u, "0:") {
		out = append(out, Finding{Category: CatRootUser, ID: CatRootUser, Location: "USER", Title: "a imagem roda como root"})
	}
	// 2. ENV com nome sensível e valor fixo
	for _, kv := range cfg.Config.Env {
		name, value, _ := strings.Cut(kv, "=")
		if SensitiveName.MatchString(name) && value != "" {
			out = append(out, Finding{Category: CatSensitiveEnv, ID: name, Location: "ENV", Title: "variável sensível com valor fixo: " + name + "=****"})
		}
	}
	// 3. Histórico: --build-arg sensível gravado e secrets em comandos
	for i, h := range cfg.History {
		if m := buildArgInHistory.FindStringSubmatch(h.CreatedBy); m != nil {
			for _, kv := range strings.Fields(m[1]) {
				name, value, _ := strings.Cut(kv, "=")
				if SensitiveName.MatchString(name) && value != "" {
					out = append(out, Finding{Category: CatSensitiveEnv, ID: name, Location: fmt.Sprintf("histórico, etapa %d", i+1), Title: "build-arg sensível gravado no histórico: " + name + "=****"})
				}
			}
		}
		for _, f := range detector.DetectString(h.CreatedBy) {
			out = append(out, Finding{Category: CatSecret, ID: f.RuleID, Location: fmt.Sprintf("histórico, etapa %d", i+1), Title: f.Description})
		}
	}
	// 4. Arquivos de todas as camadas
	for i, layer := range img.LayerInfos() {
		found, err := scanLayer(ctx, src, layer, i+1, detector)
		if err != nil {
			return nil, err
		}
		out = append(out, found...)
	}
	return out, nil
}

func scanLayer(ctx context.Context, src types.ImageSource, layer types.BlobInfo, n int, d *detect.Detector) ([]Finding, error) {
	blob, _, err := src.GetBlob(ctx, layer, none.NoCache)
	if err != nil {
		return nil, fmt.Errorf("lendo camada %d: %w", n, err)
	}
	defer blob.Close()
	rd, _, err := compression.AutoDecompress(blob)
	if err != nil {
		return nil, fmt.Errorf("descompactando camada %d: %w", n, err)
	}
	defer rd.Close()

	var out []Finding
	tr := tar.NewReader(rd)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("lendo tar da camada %d: %w", n, err)
		}
		name := strings.TrimPrefix(path.Clean("/"+hdr.Name), "/")
		if hdr.Typeflag != tar.TypeReg || hdr.Size == 0 || hdr.Size > maxFileSize || skipped(name) {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		if bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
			continue // binário
		}
		for _, f := range d.DetectBytes(data) {
			out = append(out, Finding{
				Category: CatSecret, ID: f.RuleID,
				Location: fmt.Sprintf("/%s:%d (camada %d)", name, f.StartLine+1, n),
				Title:    f.Description,
			})
		}
	}
}

func skipped(name string) bool {
	for _, d := range skipDirs {
		if strings.HasPrefix(name, d) {
			return true
		}
	}
	return false
}
