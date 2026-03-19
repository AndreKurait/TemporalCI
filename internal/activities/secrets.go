package activities

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// SecretsClient abstracts AWS Secrets Manager for testing.
type SecretsClient interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// FetchSecretsInput defines the input for the FetchSecrets activity.
type FetchSecretsInput struct {
	SecretNames []string `json:"secretNames"` // e.g. ["DOCKER_PASSWORD", "NPM_TOKEN"]
	Prefix      string   `json:"prefix"`      // e.g. "temporalci" → looks up "temporalci/DOCKER_PASSWORD"
}

// FetchSecretsResult returns resolved secret key-value pairs.
type FetchSecretsResult struct {
	Secrets map[string]string `json:"secrets"`
}

// FetchSecrets retrieves secrets from AWS Secrets Manager for injection into CI pods.
func (a *Activities) FetchSecrets(ctx context.Context, input FetchSecretsInput) (FetchSecretsResult, error) {
	if a.SecretsClient == nil {
		return FetchSecretsResult{}, fmt.Errorf("secrets manager not configured")
	}

	result := FetchSecretsResult{Secrets: make(map[string]string)}

	for _, name := range input.SecretNames {
		secretID := name
		if input.Prefix != "" {
			secretID = input.Prefix + "/" + name
		}

		out, err := a.SecretsClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
			SecretId: aws.String(secretID),
		})
		if err != nil {
			return FetchSecretsResult{}, fmt.Errorf("get secret %q: %w", secretID, err)
		}

		if out.SecretString != nil {
			// Try to parse as JSON (key-value pairs)
			var kv map[string]string
			if err := json.Unmarshal([]byte(*out.SecretString), &kv); err == nil {
				for k, v := range kv {
					result.Secrets[k] = v
				}
			} else {
				// Plain string value
				result.Secrets[name] = *out.SecretString
			}
		}
	}

	return result, nil
}
