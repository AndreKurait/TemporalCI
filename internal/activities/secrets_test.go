package activities

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type mockSecretsClient struct {
	secrets map[string]string
}

func (m *mockSecretsClient) GetSecretValue(_ context.Context, params *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	val, ok := m.secrets[*params.SecretId]
	if !ok {
		return nil, fmt.Errorf("secret not found: %s", *params.SecretId)
	}
	return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(val)}, nil
}

func TestFetchSecrets_PlainValue(t *testing.T) {
	acts := &Activities{
		SecretsClient: &mockSecretsClient{
			secrets: map[string]string{
				"myprefix/DOCKER_PASSWORD": "s3cret",
			},
		},
	}

	result, err := acts.FetchSecrets(context.Background(), FetchSecretsInput{
		SecretNames: []string{"DOCKER_PASSWORD"},
		Prefix:      "myprefix",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Secrets["DOCKER_PASSWORD"] != "s3cret" {
		t.Errorf("got %q, want s3cret", result.Secrets["DOCKER_PASSWORD"])
	}
}

func TestFetchSecrets_JSONValue(t *testing.T) {
	acts := &Activities{
		SecretsClient: &mockSecretsClient{
			secrets: map[string]string{
				"creds": `{"USER":"admin","PASS":"pw123"}`,
			},
		},
	}

	result, err := acts.FetchSecrets(context.Background(), FetchSecretsInput{
		SecretNames: []string{"creds"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Secrets["USER"] != "admin" {
		t.Errorf("USER = %q, want admin", result.Secrets["USER"])
	}
	if result.Secrets["PASS"] != "pw123" {
		t.Errorf("PASS = %q, want pw123", result.Secrets["PASS"])
	}
}

func TestFetchSecrets_NoClient(t *testing.T) {
	acts := &Activities{}
	_, err := acts.FetchSecrets(context.Background(), FetchSecretsInput{
		SecretNames: []string{"X"},
	})
	if err == nil {
		t.Error("expected error when no client")
	}
}
