package scan

import (
	"context"
	"fmt"
	"strings"

	"github.com/anchore/clio"
	"github.com/anchore/grype/grype"
	"github.com/anchore/grype/grype/db/v6/distribution"
	"github.com/anchore/grype/grype/db/v6/installation"
	"github.com/anchore/grype/grype/matcher"
	"github.com/anchore/grype/grype/pkg"
	"github.com/anchore/stereoscope/pkg/image"
	"github.com/anchore/syft/syft"
)

// scanCVEs segue a receita de cmd/grype/cli/commands/root.go (runGrype):
// carrega o banco de CVEs, cataloga os pacotes com o Syft e cruza os dois.
func scanCVEs(ctx context.Context, syftInput, dbDir string, insecure bool) ([]Finding, int, error) {
	inst := installation.DefaultConfig(clio.Identification{Name: "forja"})
	if dbDir != "" {
		inst.DBRootDir = dbDir
	}
	dist := distribution.DefaultConfig()
	dist.ID = clio.Identification{Name: "forja"}

	provider, status, err := grype.LoadVulnerabilityDB(dist, inst, true)
	if err != nil {
		return nil, 0, fmt.Errorf("carregando banco de CVEs em %s: %w", inst.DBRootDir, err)
	}
	defer provider.Close()
	if status != nil && status.Error != nil {
		return nil, 0, fmt.Errorf("banco de CVEs inválido: %w", status.Error)
	}

	sbomCfg := syft.DefaultCreateSBOMConfig()
	packages, pkgContext, _, err := pkg.Provide(syftInput, pkg.ProviderConfig{
		SyftProviderConfig: pkg.SyftProviderConfig{
			SBOMOptions: sbomCfg,
			// Só vale para scan remoto (registry:...); oci-dir não usa rede.
			RegistryOptions: &image.RegistryOptions{InsecureSkipTLSVerify: insecure, InsecureUseHTTP: insecure},
		},
	})
	if err != nil {
		return nil, 0, fmt.Errorf("catalogando pacotes: %w", err)
	}

	m := grype.VulnerabilityMatcher{
		VulnerabilityProvider: provider,
		Matchers:              matcher.NewDefaultMatchers(matcher.Config{}),
	}
	matches, _, err := m.FindMatchesContext(ctx, packages, pkgContext)
	if err != nil {
		return nil, 0, fmt.Errorf("cruzando pacotes e CVEs: %w", err)
	}

	var out []Finding
	for _, mt := range matches.Sorted() {
		v := mt.Vulnerability
		sev := "unknown"
		title := v.ID
		if v.Metadata != nil {
			if v.Metadata.Severity != "" {
				sev = strings.ToLower(v.Metadata.Severity)
			}
			if v.Metadata.Description != "" {
				title = firstLine(v.Metadata.Description, 120)
			}
		}
		out = append(out, Finding{
			Category: CatVulnerability,
			ID:       v.ID,
			Severity: sev,
			Package:  mt.Package.Name,
			Version:  mt.Package.Version,
			FixedIn:  strings.Join(v.Fix.Versions, ", "),
			Title:    title,
		})
	}
	return out, len(packages), nil
}

func firstLine(s string, limit int) string {
	s = strings.TrimSpace(strings.SplitN(s, "\n", 2)[0])
	if len(s) > limit {
		s = s[:limit] + "..."
	}
	return s
}
