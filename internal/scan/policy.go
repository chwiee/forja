package scan

import (
	_ "embed"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed defaults/policy.yaml
var defaultPolicy []byte

// Policy é a política de pontuação, lida de YAML.
type Policy struct {
	MinScore int `yaml:"min_score"`
	Weights  struct {
		Secret        int            `yaml:"secret"`
		SensitiveEnv  int            `yaml:"sensitive_env"`
		RootUser      int            `yaml:"root_user"`
		Vulnerability map[string]int `yaml:"vulnerability"`
	} `yaml:"weights"`
	Caps   map[string]int `yaml:"caps"`
	FailOn struct {
		Secrets         bool `yaml:"secrets"`
		CriticalWithFix bool `yaml:"critical_with_fix"`
	} `yaml:"fail_on"`
	Ignore []Exception `yaml:"ignore"`
}

// Exception é uma exceção: sem motivo, dono e validade ela é recusada.
type Exception struct {
	ID      string `yaml:"id"`
	Reason  string `yaml:"reason"`
	Owner   string `yaml:"owner"`
	Expires string `yaml:"expires"` // AAAA-MM-DD
}

// LoadPolicy lê a política de um arquivo; caminho vazio usa a embutida.
func LoadPolicy(path string) (*Policy, error) {
	data := defaultPolicy
	if path != "" {
		var err error
		if data, err = os.ReadFile(path); err != nil {
			return nil, fmt.Errorf("lendo política: %w", err)
		}
	}
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("política inválida: %w", err)
	}
	for i, e := range p.Ignore {
		if e.ID == "" || e.Reason == "" || e.Owner == "" || e.Expires == "" {
			return nil, fmt.Errorf("exceção %d (%q): id, reason, owner e expires são obrigatórios", i+1, e.ID)
		}
		if _, err := time.Parse("2006-01-02", e.Expires); err != nil {
			return nil, fmt.Errorf("exceção %q: expires deve ser AAAA-MM-DD", e.ID)
		}
	}
	return &p, nil
}

// Evaluate aplica exceções, pesos, tetos e regras de reprovação.
func (p *Policy) Evaluate(r *Report, now time.Time) {
	var kept []Finding
	for _, f := range r.Findings {
		if e, ok := p.exceptionFor(f, now); ok {
			r.Ignored = append(r.Ignored, Ignored{Finding: f, Reason: e.Reason, Owner: e.Owner, Expires: e.Expires})
			continue
		}
		f.Penalty = p.weight(f)
		kept = append(kept, f)
	}
	r.Findings = kept

	perCategory := map[string]int{}
	for _, f := range kept {
		perCategory[f.Category] += f.Penalty
	}
	total := 0
	for cat, sum := range perCategory {
		if c, ok := p.Caps[cat]; ok && sum > c {
			sum = c
		}
		total += sum
	}
	r.Score = max(0, 100-total)
	r.MinScore = p.MinScore

	r.Reasons = nil
	if r.Score < p.MinScore {
		r.Reasons = append(r.Reasons, fmt.Sprintf("nota %d abaixo do mínimo %d", r.Score, p.MinScore))
	}
	for _, f := range kept {
		if p.FailOn.Secrets && f.Category == CatSecret {
			r.Reasons = append(r.Reasons, "secret encontrado: "+f.ID+" em "+f.Location)
		}
		if p.FailOn.CriticalWithFix && f.Category == CatVulnerability && f.Severity == "critical" && f.FixedIn != "" {
			r.Reasons = append(r.Reasons, fmt.Sprintf("%s crítica com correção (%s %s → %s)", f.ID, f.Package, f.Version, f.FixedIn))
		}
	}
	r.Passed = len(r.Reasons) == 0

	sort.SliceStable(r.Findings, func(i, j int) bool { return r.Findings[i].Penalty > r.Findings[j].Penalty })
}

func (p *Policy) weight(f Finding) int {
	switch f.Category {
	case CatSecret:
		return p.Weights.Secret
	case CatSensitiveEnv:
		return p.Weights.SensitiveEnv
	case CatRootUser:
		return p.Weights.RootUser
	case CatVulnerability:
		return p.Weights.Vulnerability[strings.ToLower(f.Severity)]
	}
	return 0
}

// exceptionFor devolve a exceção que cobre o achado, se ainda estiver válida.
// A exceção casa pelo id do achado (CVE, regra, variável) ou pela categoria.
func (p *Policy) exceptionFor(f Finding, now time.Time) (Exception, bool) {
	for _, e := range p.Ignore {
		if e.ID != f.ID && e.ID != f.Category {
			continue
		}
		exp, _ := time.Parse("2006-01-02", e.Expires)
		if now.Before(exp.Add(24 * time.Hour)) {
			return e, true
		}
	}
	return Exception{}, false
}
