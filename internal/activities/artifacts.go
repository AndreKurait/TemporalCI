package activities

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// UploadArtifactInput defines input for artifact upload.
type UploadArtifactInput struct {
	WorkflowID string           `json:"workflowID"`
	StepName   string           `json:"stepName"`
	Repo       string           `json:"repo"`
	Paths      []ArtifactUpload `json:"paths"`
}

// UploadArtifactResult returns URLs of uploaded artifacts.
type UploadArtifactResult struct {
	URLs []string `json:"urls"`
}

// UploadArtifacts uploads step artifacts to S3.
func (a *Activities) UploadArtifacts(ctx context.Context, input UploadArtifactInput) (UploadArtifactResult, error) {
	if a.S3Client == nil || a.LogBucket == "" {
		return UploadArtifactResult{}, nil
	}

	var urls []string
	for _, art := range input.Paths {
		matches, _ := filepath.Glob(art.Path)
		if len(matches) == 0 {
			matches = []string{art.Path}
		}
		for _, path := range matches {
			f, err := os.Open(path)
			if err != nil {
				a.logger(ctx).Warn("artifact not found", "path", path, "error", err)
				continue
			}
			key := fmt.Sprintf("artifacts/%s/%s/%s/%s", input.Repo, input.WorkflowID, input.StepName, filepath.Base(path))
			_, err = a.S3Client.PutObject(ctx, &s3.PutObjectInput{
				Bucket: &a.LogBucket, Key: &key, Body: f,
			})
			f.Close()
			if err != nil {
				return UploadArtifactResult{}, fmt.Errorf("upload artifact %s: %w", path, err)
			}
			if a.S3Presigner != nil {
				presigned, err := a.S3Presigner.PresignGetObject(ctx, &s3.GetObjectInput{
					Bucket: &a.LogBucket, Key: &key,
				}, func(o *s3.PresignOptions) { o.Expires = 24 * time.Hour })
				if err == nil {
					urls = append(urls, presigned.URL)
				}
			}
		}
	}
	return UploadArtifactResult{URLs: urls}, nil
}

// DownloadArtifactInput defines input for artifact download.
type DownloadArtifactInput struct {
	WorkflowID string `json:"workflowID"`
	FromStep   string `json:"fromStep"`
	Repo       string `json:"repo"`
	DestDir    string `json:"destDir"`
}

// DownloadArtifacts downloads artifacts from S3 to a local directory.
func (a *Activities) DownloadArtifacts(ctx context.Context, input DownloadArtifactInput) error {
	if a.S3Full == nil || a.LogBucket == "" {
		return nil
	}
	prefix := fmt.Sprintf("artifacts/%s/%s/%s/", input.Repo, input.WorkflowID, input.FromStep)
	listOut, err := a.S3Full.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: &a.LogBucket, Prefix: &prefix,
	})
	if err != nil {
		return fmt.Errorf("list artifacts: %w", err)
	}
	_ = os.MkdirAll(input.DestDir, 0755)
	for _, obj := range listOut.Contents {
		getOut, err := a.S3Full.GetObject(ctx, &s3.GetObjectInput{
			Bucket: &a.LogBucket, Key: obj.Key,
		})
		if err != nil {
			return fmt.Errorf("download %s: %w", aws.ToString(obj.Key), err)
		}
		dest := filepath.Join(input.DestDir, filepath.Base(aws.ToString(obj.Key)))
		f, err := os.Create(dest)
		if err != nil {
			getOut.Body.Close()
			return err
		}
		_, err = io.Copy(f, getOut.Body)
		getOut.Body.Close()
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// ListArtifactsInput defines input for listing artifacts.
type ListArtifactsInput struct {
	Repo       string `json:"repo"`
	WorkflowID string `json:"workflowID"`
}

// ArtifactInfo describes a single artifact.
type ArtifactInfo struct {
	Step string `json:"step"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// ListArtifactsResult returns artifact metadata.
type ListArtifactsResult struct {
	Artifacts []ArtifactInfo `json:"artifacts"`
}

// ListArtifacts lists all artifacts for a pipeline run.
func (a *Activities) ListArtifacts(ctx context.Context, input ListArtifactsInput) (ListArtifactsResult, error) {
	if a.S3Full == nil || a.LogBucket == "" {
		return ListArtifactsResult{}, nil
	}
	prefix := fmt.Sprintf("artifacts/%s/%s/", input.Repo, input.WorkflowID)
	listOut, err := a.S3Full.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: &a.LogBucket, Prefix: &prefix,
	})
	if err != nil {
		return ListArtifactsResult{}, err
	}
	var result ListArtifactsResult
	for _, obj := range listOut.Contents {
		key := aws.ToString(obj.Key)
		parts := strings.Split(strings.TrimPrefix(key, prefix), "/")
		step := ""
		if len(parts) >= 2 {
			step = parts[0]
		}
		result.Artifacts = append(result.Artifacts, ArtifactInfo{
			Step: step, Name: filepath.Base(key), Size: *obj.Size,
		})
	}
	return result, nil
}
