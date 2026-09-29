// Package scan analisa uma imagem (CVEs, secrets, configuração) e calcula
// uma nota de 0 a 100 segundo uma política versionada no repositório.
package scan

import "time"

// Categorias de achado. Também servem de id em exceções genéricas (root_user).
const (
	CatVulnerability = "vulnerability"
	CatSecret        = "secret"
	CatSensitiveEnv  = "sensitive_env"
	CatRootUser      = "root_user"
)

// Finding é um achado, de qualquer scanner.
type Finding struct {
	Category string `json:"category"`
	ID       string `json:"id"`                 // CVE-..., regra do gitleaks, nome da variável
	Severity string `json:"severity,omitempty"` // critical, high, medium, low, negligible, unknown
	Package  string `json:"package,omitempty"`
	Version  string `json:"version,omitempty"`
	FixedIn  string `json:"fixed_in,omitempty"`
	Location string `json:"location,omitempty"` // arquivo na imagem, ENV, histórico
	Title    string `json:"title"`
	Penalty  int    `json:"penalty"`
}

// Ignored é um achado que casou com uma exceção válida da política.
type Ignored struct {
	Finding
	Reason  string `json:"reason"`
	Owner   string `json:"owner"`
	Expires string `json:"expires"`
}

// Report é o resultado final de um scan.
type Report struct {
	Image     string    `json:"image"`
	ScannedAt time.Time `json:"scanned_at"`
	Findings  []Finding `json:"findings"`
	Ignored   []Ignored `json:"ignored"`
	Score     int       `json:"score"`
	MinScore  int       `json:"min_score"`
	Passed    bool      `json:"passed"`
	Reasons   []string  `json:"reasons"` // por que reprovou
	Packages  int       `json:"packages"`
}
