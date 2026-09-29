package scan

import (
	"encoding/json"
	"io"
	"strings"
)

// WriteSARIF grava o relatório no formato SARIF 2.1.0, que a aba Security do
// GitHub (upload-sarif) e a maioria das ferramentas de segurança entendem.
func WriteSARIF(w io.Writer, r *Report, version string) error {
	type msg struct {
		Text string `json:"text"`
	}
	type rule struct {
		ID               string `json:"id"`
		ShortDescription msg    `json:"shortDescription"`
	}
	type location struct {
		PhysicalLocation struct {
			ArtifactLocation struct {
				URI string `json:"uri"`
			} `json:"artifactLocation"`
		} `json:"physicalLocation"`
	}
	type result struct {
		RuleID    string     `json:"ruleId"`
		Level     string     `json:"level"`
		Message   msg        `json:"message"`
		Locations []location `json:"locations"`
	}

	rules := map[string]rule{}
	var results []result
	for _, f := range r.Findings {
		ruleID := f.Category + "/" + f.ID
		if _, ok := rules[ruleID]; !ok {
			rules[ruleID] = rule{ID: ruleID, ShortDescription: msg{f.Title}}
		}
		var loc location
		// O GitHub exige um caminho de arquivo; o Dockerfile é o mais útil.
		loc.PhysicalLocation.ArtifactLocation.URI = "Dockerfile"
		text := f.Title
		if f.Location != "" {
			text += " (" + f.Location + ")"
		}
		results = append(results, result{RuleID: ruleID, Level: sarifLevel(f), Message: msg{text}, Locations: []location{loc}})
	}
	ruleList := make([]rule, 0, len(rules))
	for _, ru := range rules {
		ruleList = append(ruleList, ru)
	}

	doc := map[string]any{
		"$schema": "https://json.schemastore.org/sarif-2.1.0.json",
		"version": "2.1.0",
		"runs": []any{map[string]any{
			"tool": map[string]any{"driver": map[string]any{
				"name": "forja", "version": version, "informationUri": "https://github.com/chwiee/forja", "rules": ruleList,
			}},
			"results": results,
		}},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

func sarifLevel(f Finding) string {
	switch {
	case f.Category == CatSecret, strings.EqualFold(f.Severity, "critical"), strings.EqualFold(f.Severity, "high"):
		return "error"
	case f.Category == CatSensitiveEnv, strings.EqualFold(f.Severity, "medium"):
		return "warning"
	}
	return "note"
}
