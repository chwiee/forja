package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/chwiee/forja/internal/build"
	"github.com/chwiee/forja/internal/scan"
)

// hostOS existe para o teste simular Linux quando roda no Windows.
var hostOS = runtime.GOOS

// Scanner é o contrato do scan. O main passa scan.Scan; o teste, um fake.
type Scanner func(ctx context.Context, t scan.Target, o scan.Options) (*scan.Report, error)

// scanFlags são as flags comuns a scan e run.
type scanFlags struct {
	policy   string
	sarif    string
	dbDir    string
	skipCVEs bool
}

func (f *scanFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.policy, "policy", "", "arquivo de política (padrão: a embutida, nota mínima 70)")
	fl.StringVar(&f.sarif, "sarif", "", "grava também o relatório em SARIF neste arquivo (aba Security do GitHub)")
	fl.StringVar(&f.dbDir, "db-dir", envOr("FORJA_DB_DIR", defaultDBDir()), "cache do banco de CVEs (env FORJA_DB_DIR)")
	fl.BoolVar(&f.skipCVEs, "skip-cves", false, "não procura CVEs (sem internet); secrets e config continuam")
}

// defaultDBDir: no Linux (container), /var/tmp já é um volume gravável.
func defaultDBDir() string {
	if runtime.GOOS == "linux" {
		return "/var/tmp/forja-db"
	}
	return "" // padrão do Grype: cache do usuário
}

func newScanCmd(opts *options) *cobra.Command {
	var (
		sf     scanFlags
		remote bool
	)
	cmd := &cobra.Command{
		Use:   "scan IMAGEM",
		Short: "Procura CVEs, secrets e configuração insegura e dá uma nota (exit 3 se reprovar)",
		Long: `Escaneia uma imagem e calcula uma nota de 0 a 100 segundo a política.

Sem --remote, a imagem vem do storage local (buildada antes com forja build):
ela é exportada para um diretório OCI e escaneada ali, ou seja, exatamente os
bytes que o push vai publicar. Imagens multi-arch: cada arquitetura é
escaneada separadamente e todas precisam passar.

Exit codes: 0 aprovada | 3 reprovada no gate | 1 erro`,
		Example: `  forja scan registry.local/app:1.0
  forja scan --remote --policy forja-policy.yaml --sarif forja.sarif ghcr.io/org/app:1.0`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := opts.ctx(cmd)
			defer cancel()
			return runScan(ctx, cmd, opts, sf, args[0], remote)
		},
	}
	sf.register(cmd)
	cmd.Flags().BoolVar(&remote, "remote", false, "escaneia direto do registry, sem storage local")
	return cmd
}

// runScan é usado por scan e por run.
func runScan(ctx context.Context, cmd *cobra.Command, opts *options, sf scanFlags, image string, remote bool) error {
	// Testado: no Windows nativo, a biblioteca de camadas do Syft grava arquivos
	// com ":" no nome (0-sha256:...), que o Windows recusa. Melhor avisar já.
	if hostOS == "windows" {
		return errors.New("scan não funciona no Windows nativo (a biblioteca do Syft usa nomes de arquivo inválidos no Windows); rode a imagem do forja com Docker")
	}
	policy, err := scan.LoadPolicy(sf.policy)
	if err != nil {
		return err
	}
	var targets []scan.Target
	if remote {
		targets = append(targets, scan.Target{Name: image, Remote: image, TLSVerify: opts.tlsVerify})
	} else {
		// Exporta do storage local: escaneia os mesmos bytes que o push publica.
		exported, err := opts.deps.Engine.Export(ctx, build.ExportOptions{
			Image: image, Dir: "/var/tmp/forja-scan", StorageDriver: opts.storageDriver,
		})
		if err != nil {
			return err
		}
		for _, e := range exported {
			name := image
			if e.Platform != "" {
				name += " (" + e.Platform + ")"
			}
			targets = append(targets, scan.Target{Name: name, OCIDir: e.Dir, TLSVerify: opts.tlsVerify})
		}
	}

	var reports []*scan.Report
	passed := true
	for _, t := range targets {
		fmt.Fprintf(cmd.ErrOrStderr(), "escaneando %s...\n", t.Name)
		r, err := opts.deps.Scanner(ctx, t, scan.Options{DBDir: sf.dbDir, SkipCVEs: sf.skipCVEs})
		if err != nil {
			return err
		}
		policy.Evaluate(r, time.Now())
		passed = passed && r.Passed
		reports = append(reports, r)
	}

	if sf.sarif != "" {
		if err := writeSARIF(sf.sarif, reports); err != nil {
			return err
		}
	}
	out := cmd.OutOrStdout()
	if opts.output == "json" {
		if err := json.NewEncoder(out).Encode(reports); err != nil {
			return err
		}
	} else {
		for _, r := range reports {
			printReport(out, r)
		}
	}
	if !passed {
		return &ExitError{Code: ExitFindings, Err: fmt.Errorf("%s reprovada no gate de segurança", image)}
	}
	return nil
}

func writeSARIF(path string, reports []*scan.Report) error {
	merged := &scan.Report{}
	for _, r := range reports {
		merged.Findings = append(merged.Findings, r.Findings...)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return scan.WriteSARIF(f, merged, version)
}

const maxRows = 15

func printReport(w io.Writer, r *scan.Report) {
	status := "APROVADA"
	if !r.Passed {
		status = "REPROVADA"
	}
	fmt.Fprintf(w, "\nSCAN  %s\nNOTA  %d/100 (mínimo %d)  %s   pacotes: %d  achados: %d  exceções: %d\n",
		r.Image, r.Score, r.MinScore, status, r.Packages, len(r.Findings), len(r.Ignored))
	if len(r.Findings) > 0 {
		tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "  TIPO\tID\tSEVERIDADE\tONDE\tDESCONTO")
		for i, f := range r.Findings {
			if i == maxRows {
				fmt.Fprintf(tw, "  ...\te mais %d achados (use -o json)\t\t\t\n", len(r.Findings)-maxRows)
				break
			}
			where := f.Location
			if f.Package != "" {
				where = f.Package + " " + f.Version
				if f.FixedIn != "" {
					where += " → " + f.FixedIn
				}
			}
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t-%d\n", f.Category, f.ID, dash(f.Severity), where, f.Penalty)
		}
		tw.Flush()
	}
	for _, reason := range r.Reasons {
		fmt.Fprintf(w, "  REPROVOU: %s\n", reason)
	}
	for _, ig := range r.Ignored {
		fmt.Fprintf(w, "  exceção: %s (%s, dono %s, até %s)\n", ig.ID, ig.Reason, ig.Owner, ig.Expires)
	}
}

func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
