package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/chwiee/forja/internal/scan"
)

type finding struct {
	Variable string `json:"variable"`
	Reason   string `json:"reason"`
}

func newInspectCmd(opts *options) *cobra.Command {
	var failOnFindings bool

	cmd := &cobra.Command{
		Use:   "inspect IMAGEM",
		Short: "Mostra USER/ENV/LABELS e aponta variáveis sensíveis hardcoded",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := opts.ctx(cmd)
			defer cancel()

			info, err := opts.client.Inspect(ctx, args[0])
			if err != nil {
				return err
			}

			var findings []finding
			for _, kv := range info.Env {
				name, value, _ := strings.Cut(kv, "=")
				if scan.SensitiveName.MatchString(name) && value != "" {
					findings = append(findings, finding{name, "nome sensível com valor fixo na imagem"})
				}
			}
			if info.User == "" || info.User == "root" || info.User == "0" {
				findings = append(findings, finding{"USER", "imagem roda como root"})
			}

			out := cmd.OutOrStdout()
			if opts.output == "json" {
				_ = json.NewEncoder(out).Encode(map[string]any{"image": args[0], "config": info, "findings": findings})
			} else {
				tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
				fmt.Fprintf(tw, "IMAGEM\t%s\nCONFIG DIGEST\t%s\nUSER\t%q\n", args[0], info.Digest, info.User)
				for _, e := range info.Env {
					fmt.Fprintf(tw, "ENV\t%s\n", mask(e))
				}
				for _, f := range findings {
					fmt.Fprintf(tw, "ACHADO\t%s: %s\n", f.Variable, f.Reason)
				}
				tw.Flush()
			}

			if failOnFindings && len(findings) > 0 {
				return &ExitError{Code: ExitFindings, Err: fmt.Errorf("%d achado(s) em %s", len(findings), args[0])}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&failOnFindings, "fail-on-findings", false, "sai com código 3 se houver achados (gate de CI)")
	return cmd
}

// mask esconde o valor de variáveis sensíveis: nunca imprima segredo em log de CI.
func mask(kv string) string {
	name, value, ok := strings.Cut(kv, "=")
	if ok && scan.SensitiveName.MatchString(name) && value != "" {
		return name + "=****"
	}
	return kv
}
