// Package registries resolve nomes curtos (--registry ecr) em endereços de
// imagem e obtém as credenciais de cada tipo de registry.
package registries

import (
	_ "embed"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed defaults/registries.yaml
var defaultConfig []byte

// Tipos de registry.
const (
	TypeECR    = "ecr"    // credencial via SDK da AWS
	TypeDocker = "docker" // credencial do docker login / REGISTRY_AUTH_FILE
)

// Registry é um registry configurado.
type Registry struct {
	Name    string `yaml:"-"`
	Type    string `yaml:"type"`
	Host    string `yaml:"host"`
	Account string `yaml:"account"`
	Region  string `yaml:"region"`
}

// Load lê a configuração embutida ou, se path não for vazio, a do arquivo.
// Variáveis de ambiente sobrescrevem conta e região do ECR.
func Load(path string) (map[string]Registry, error) {
	data := defaultConfig
	if path != "" {
		var err error
		if data, err = os.ReadFile(path); err != nil {
			return nil, fmt.Errorf("lendo registries: %w", err)
		}
	}
	var regs map[string]Registry
	if err := yaml.Unmarshal(data, &regs); err != nil {
		return nil, fmt.Errorf("registries inválido: %w", err)
	}
	for name, r := range regs {
		r.Name = name
		if r.Type == TypeECR {
			if v := os.Getenv("FORJA_ECR_ACCOUNT"); v != "" {
				r.Account = v
			}
			if v := os.Getenv("FORJA_ECR_REGION"); v != "" {
				r.Region = v
			}
			if v := os.Getenv("FORJA_ECR_HOST"); v != "" {
				r.Host = v // ex.: emulador local (floci)
			}
		}
		regs[name] = r
	}
	return regs, nil
}

// Lookup devolve o registry pelo nome, com o host calculado e validado.
func Lookup(regs map[string]Registry, name string) (Registry, error) {
	r, ok := regs[name]
	if !ok {
		names := make([]string, 0, len(regs))
		for n := range regs {
			names = append(names, n)
		}
		sort.Strings(names)
		return Registry{}, fmt.Errorf("registry %q desconhecido; conhecidos: %s", name, strings.Join(names, ", "))
	}
	switch r.Type {
	case TypeECR:
		if r.Region == "" {
			return Registry{}, fmt.Errorf("registry %q: região não configurada", name)
		}
		if r.Host == "" {
			if r.Account == "" {
				return Registry{}, fmt.Errorf("registry %q: conta AWS não configurada (defina FORJA_ECR_ACCOUNT ou account no arquivo de registries)", name)
			}
			r.Host = fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", r.Account, r.Region)
		}
	case TypeDocker:
		if r.Host == "" {
			return Registry{}, fmt.Errorf("registry %q: host não configurado", name)
		}
	default:
		return Registry{}, fmt.Errorf("registry %q: tipo %q desconhecido (use ecr ou docker)", name, r.Type)
	}
	return r, nil
}

// Reference monta "host/nome:tag" com nome e tag normalizados.
func (r Registry) Reference(name, tag string) (string, error) {
	n, err := NormalizeName(name)
	if err != nil {
		return "", err
	}
	t, err := NormalizeTag(tag)
	if err != nil {
		return "", err
	}
	return r.Host + "/" + n + ":" + t, nil
}

var (
	validName = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*$`)
	badTag    = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)
	pullRef   = regexp.MustCompile(`^refs/pull/(\d+)/(?:merge|head)$`)
)

// NormalizeName aceita ${{ github.repository }} ("Org/Repo") e devolve o
// nome em minúsculas, como ECR e GHCR exigem.
func NormalizeName(name string) (string, error) {
	n := strings.Trim(strings.ToLower(strings.TrimSpace(name)), "/")
	if n == "" {
		return "", fmt.Errorf("--name vazio")
	}
	if !validName.MatchString(n) {
		return "", fmt.Errorf("nome de repositório inválido: %q", name)
	}
	return n, nil
}

// NormalizeTag aceita ${{ github.ref }} ou ${{ github.ref_name }}:
//
//	refs/tags/v1.2.0        → v1.2.0
//	refs/heads/feature/x    → feature-x
//	refs/pull/42/merge      → pr-42
//
// Caracteres inválidos viram "-", e o resultado tem no máximo 128 caracteres.
func NormalizeTag(tag string) (string, error) {
	t := strings.TrimSpace(tag)
	if m := pullRef.FindStringSubmatch(t); m != nil {
		t = "pr-" + m[1]
	}
	t = strings.TrimPrefix(t, "refs/tags/")
	t = strings.TrimPrefix(t, "refs/heads/")
	t = badTag.ReplaceAllString(t, "-")
	t = strings.TrimLeft(t, ".-")
	if len(t) > 128 {
		t = t[:128]
	}
	if t == "" {
		return "", fmt.Errorf("--tag vazia ou inválida: %q", tag)
	}
	return t, nil
}
