package activities

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// AssumeRoleInput defines input for the AssumeRole activity.
type AssumeRoleInput struct {
	RoleARN     string `json:"roleARN"`
	SessionName string `json:"sessionName"`
	Duration    int32  `json:"duration"` // seconds, default 3600
	// For chained assumption: use these as source credentials
	SourceAccessKey    string `json:"sourceAccessKey,omitempty"`
	SourceSecretKey    string `json:"sourceSecretKey,omitempty"`
	SourceSessionToken string `json:"sourceSessionToken,omitempty"`
}

// AssumeRoleResult returns temporary credentials.
type AssumeRoleResult struct {
	AccessKeyID     string `json:"accessKeyID"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken"`
	Expiration      string `json:"expiration"`
}

// AssumeRole assumes an IAM role via STS and returns temporary credentials.
// Supports credential chaining (source credentials from a previous assumption).
func (a *Activities) AssumeRole(ctx context.Context, input AssumeRoleInput) (AssumeRoleResult, error) {
	if input.Duration == 0 {
		input.Duration = 3600
	}
	if input.SessionName == "" {
		input.SessionName = "temporalci"
	}

	var cfg aws.Config
	var err error

	if input.SourceAccessKey != "" {
		// Chained assumption: use provided credentials
		cfg, err = awsconfig.LoadDefaultConfig(ctx,
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				input.SourceAccessKey, input.SourceSecretKey, input.SourceSessionToken,
			)),
		)
	} else {
		// Use default credentials (Pod Identity / instance role)
		cfg, err = awsconfig.LoadDefaultConfig(ctx)
	}
	if err != nil {
		return AssumeRoleResult{}, fmt.Errorf("load AWS config: %w", err)
	}

	client := sts.NewFromConfig(cfg)
	result, err := client.AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn:         &input.RoleARN,
		RoleSessionName: &input.SessionName,
		DurationSeconds: &input.Duration,
	})
	if err != nil {
		return AssumeRoleResult{}, fmt.Errorf("assume role %s: %w", input.RoleARN, err)
	}

	return AssumeRoleResult{
		AccessKeyID:     *result.Credentials.AccessKeyId,
		SecretAccessKey: *result.Credentials.SecretAccessKey,
		SessionToken:    *result.Credentials.SessionToken,
		Expiration:      result.Credentials.Expiration.String(),
	}, nil
}
