package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var today = time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)

func TestEvaluate(t *testing.T) {
	p, err := LoadPolicy("") // política embutida
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		findings   []Finding
		wantScore  int
		wantPassed bool
		wantReason string
	}{
		{"imagem limpa", nil, 100, true, ""},
		{"root só desconta", []Finding{{Category: CatRootUser, ID: CatRootUser}}, 95, true, ""},
		{"secret reprova sempre", []Finding{{Category: CatSecret, ID: "github-pat", Location: "/etc/app.conf"}}, 60, false, "secret encontrado"},
		{"crítica com fix reprova", []Finding{{Category: CatVulnerability, ID: "CVE-1", Severity: "critical", FixedIn: "1.2"}}, 85, false, "crítica com correção"},
		{"crítica sem fix só desconta", []Finding{{Category: CatVulnerability, ID: "CVE-1", Severity: "critical"}}, 85, true, ""},
		{"teto de CVEs", manyHighs(40), 40, false, "abaixo do mínimo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Report{Findings: tt.findings}
			p.Evaluate(r, today)
			if r.Score != tt.wantScore || r.Passed != tt.wantPassed {
				t.Errorf("score=%d passed=%v, queria %d/%v (motivos: %v)", r.Score, r.Passed, tt.wantScore, tt.wantPassed, r.Reasons)
			}
			if tt.wantReason != "" && !strings.Contains(strings.Join(r.Reasons, "|"), tt.wantReason) {
				t.Errorf("motivos %v não contêm %q", r.Reasons, tt.wantReason)
			}
		})
	}
}

func manyHighs(n int) []Finding {
	var out []Finding
	for i := 0; i < n; i++ {
		out = append(out, Finding{Category: CatVulnerability, ID: "CVE-X", Severity: "high"})
	}
	return out // 40 × 5 = 200, mas o teto de vulnerability é 60
}

func TestExceptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.yaml")
	os.WriteFile(path, []byte(`min_score: 70
weights: {secret: 40, root_user: 5}
fail_on: {secrets: true}
ignore:
  - {id: root_user, reason: "entrypoint troca de usuário", owner: dados, expires: 2026-12-31}
  - {id: github-pat, reason: "token de teste", owner: seg, expires: 2026-01-01}
`), 0o644)
	p, err := LoadPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	r := &Report{Findings: []Finding{
		{Category: CatRootUser, ID: CatRootUser},
		{Category: CatSecret, ID: "github-pat"},
	}}
	p.Evaluate(r, today)
	if len(r.Ignored) != 1 || r.Ignored[0].ID != CatRootUser {
		t.Errorf("só a exceção válida deveria valer; ignorados: %+v", r.Ignored)
	}
	if r.Passed {
		t.Error("exceção vencida não pode liberar o secret")
	}
}

func TestExceptionNeedsOwnerAndExpiry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.yaml")
	os.WriteFile(path, []byte("ignore:\n  - {id: CVE-1, reason: x}\n"), 0o644)
	if _, err := LoadPolicy(path); err == nil || !strings.Contains(err.Error(), "obrigatórios") {
		t.Errorf("queria erro de campos obrigatórios, veio %v", err)
	}
}
