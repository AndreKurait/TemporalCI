package activities

import (
	"context"
	"testing"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// mockS3Client implements S3Uploader for testing.
type mockS3Client struct {
	putCalled bool
	putBucket string
	putKey    string
}

func (m *mockS3Client) PutObject(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.putCalled = true
	m.putBucket = *params.Bucket
	m.putKey = *params.Key
	return &s3.PutObjectOutput{}, nil
}

// mockPresigner implements S3Presigner for testing.
type mockPresigner struct {
	url string
}

func (m *mockPresigner) PresignGetObject(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	return &v4.PresignedHTTPRequest{URL: m.url}, nil
}

func TestUploadLog_NoS3(t *testing.T) {
	acts := &Activities{}
	result, err := acts.UploadLog(context.Background(), UploadLogInput{
		WorkflowID: "wf-1", StepName: "build", Logs: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.LogURL != "" {
		t.Errorf("expected empty URL when no S3 client, got %q", result.LogURL)
	}
}

func TestUploadLog_WithS3(t *testing.T) {
	mock := &mockS3Client{}
	presigner := &mockPresigner{url: "https://s3.example.com/logs/wf-1/build.log?signed"}

	acts := &Activities{
		S3Client:    mock,
		S3Presigner: presigner,
		LogBucket:   "test-bucket",
	}

	result, err := acts.UploadLog(context.Background(), UploadLogInput{
		WorkflowID: "wf-1", StepName: "build", Logs: "build output here",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !mock.putCalled {
		t.Error("expected PutObject to be called")
	}
	if mock.putBucket != "test-bucket" {
		t.Errorf("bucket = %q, want test-bucket", mock.putBucket)
	}
	if mock.putKey != "logs/wf-1/build.log" {
		t.Errorf("key = %q, want logs/wf-1/build.log", mock.putKey)
	}
	if result.LogURL != "https://s3.example.com/logs/wf-1/build.log?signed" {
		t.Errorf("URL = %q, want presigned URL", result.LogURL)
	}
}

func TestUploadLog_NoBucket(t *testing.T) {
	acts := &Activities{
		S3Client: &mockS3Client{},
		// LogBucket is empty
	}
	result, err := acts.UploadLog(context.Background(), UploadLogInput{
		WorkflowID: "wf-1", StepName: "build", Logs: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.LogURL != "" {
		t.Errorf("expected empty URL when no bucket, got %q", result.LogURL)
	}
}
