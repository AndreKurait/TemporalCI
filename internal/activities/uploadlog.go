package activities

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

// S3Uploader abstracts S3 operations for testing.
type S3Uploader interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// S3Presigner abstracts S3 presign operations for testing.
type S3Presigner interface {
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// UploadLogInput defines the input for the UploadLog activity.
type UploadLogInput struct {
	WorkflowID string `json:"workflowID"`
	StepName   string `json:"stepName"`
	Logs       string `json:"logs"`
}

// UploadLogResult defines the output of the UploadLog activity.
type UploadLogResult struct {
	LogURL string `json:"logURL"`
}

// UploadLog uploads full step logs to S3 and returns a presigned URL.
func (a *Activities) UploadLog(ctx context.Context, input UploadLogInput) (UploadLogResult, error) {
	if a.S3Client == nil || a.LogBucket == "" {
		return UploadLogResult{}, nil
	}

	key := fmt.Sprintf("logs/%s/%s.log", input.WorkflowID, input.StepName)

	_, err := a.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(a.LogBucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader([]byte(input.Logs)),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		return UploadLogResult{}, fmt.Errorf("s3 put: %w", err)
	}

	if a.S3Presigner == nil {
		return UploadLogResult{}, nil
	}

	presigned, err := a.S3Presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(a.LogBucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = 1 * time.Hour
	})
	if err != nil {
		return UploadLogResult{}, fmt.Errorf("presign: %w", err)
	}

	return UploadLogResult{LogURL: presigned.URL}, nil
}
