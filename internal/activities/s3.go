package activities

import (
	"context"
	"fmt"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.temporal.io/sdk/activity"
)

// UploadLog uploads step output to S3 and returns a presigned URL.
func (a *Activities) UploadLog(ctx context.Context, input UploadLogInput) (UploadLogResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Uploading log", "workflowID", input.WorkflowID, "activityID", input.ActivityID)

	if a.LogBucket == "" {
		return UploadLogResult{}, fmt.Errorf("LOG_BUCKET not configured")
	}

	opts := []func(*awsconfig.LoadOptions) error{}
	if a.AWSRegion != "" {
		opts = append(opts, awsconfig.WithRegion(a.AWSRegion))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return UploadLogResult{}, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(cfg)
	key := fmt.Sprintf("logs/%s/%s.log", input.WorkflowID, input.ActivityID)

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &a.LogBucket,
		Key:    &key,
		Body:   strings.NewReader(input.Content),
	})
	if err != nil {
		return UploadLogResult{}, fmt.Errorf("s3 put: %w", err)
	}

	presigner := s3.NewPresignClient(client)
	presigned, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &a.LogBucket,
		Key:    &key,
	}, s3.WithPresignExpires(1*time.Hour))
	if err != nil {
		return UploadLogResult{}, fmt.Errorf("presign: %w", err)
	}

	return UploadLogResult{URL: presigned.URL}, nil
}
