package registries

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

// Credentials é usuário e senha para o registry. Nil significa "use o
// arquivo de credenciais do docker login / REGISTRY_AUTH_FILE".
type Credentials struct {
	Username string
	Password string
}

// Credentials obtém a credencial do registry.
//
// Para o ECR, usa a cadeia padrão do SDK da AWS, que cobre, nesta ordem de
// uso típico: EKS Pod Identity (AWS_CONTAINER_CREDENTIALS_FULL_URI), IRSA e
// OIDC do GitHub (AWS_WEB_IDENTITY_TOKEN_FILE), e variáveis AWS_ACCESS_KEY_ID.
// AWS_ENDPOINT_URL aponta para um emulador (floci) nos testes.
func (r Registry) Credentials(ctx context.Context) (*Credentials, error) {
	if r.Type != TypeECR {
		return nil, nil
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(r.Region))
	if err != nil {
		return nil, fmt.Errorf("configuração AWS: %w", err)
	}
	out, err := ecr.NewFromConfig(cfg).GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return nil, fmt.Errorf("pedindo token do ECR (confira a role/Pod Identity): %w", err)
	}
	if len(out.AuthorizationData) == 0 || out.AuthorizationData[0].AuthorizationToken == nil {
		return nil, errors.New("o ECR não devolveu token")
	}
	return decodeToken(*out.AuthorizationData[0].AuthorizationToken)
}

// decodeToken abre o token do ECR: base64("AWS:<senha>").
func decodeToken(token string) (*Credentials, error) {
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("token do ECR inválido: %w", err)
	}
	user, pass, ok := strings.Cut(string(raw), ":")
	if !ok || user == "" || pass == "" {
		return nil, errors.New("token do ECR fora do formato usuário:senha")
	}
	return &Credentials{Username: user, Password: pass}, nil
}
