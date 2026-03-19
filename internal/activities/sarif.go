package activities

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/google/go-github/v70/github"
)

// UploadSARIFInput defines input for the UploadSARIF activity.
type UploadSARIFInput struct {
	Repo      string `json:"repo"`
	HeadSHA   string `json:"headSHA"`
	Ref       string `json:"ref"`
	SARIFPath string `json:"sarifPath"`
}

// UploadSARIF uploads a SARIF file to GitHub's code scanning API.
func (a *Activities) UploadSARIF(ctx context.Context, input UploadSARIFInput) error {
	gh, err := a.githubClient(ctx, input.Repo)
	if err != nil || gh == nil {
		return err
	}
	owner, repo, err := splitRepo(input.Repo)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(input.SARIFPath)
	if err != nil {
		return fmt.Errorf("read SARIF: %w", err)
	}

	// Gzip + base64 encode
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(data); err != nil {
		return fmt.Errorf("gzip SARIF: %w", err)
	}
	gw.Close()
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	_, _, err = gh.CodeScanning.UploadSarif(ctx, owner, repo, &github.SarifAnalysis{
		CommitSHA: &input.HeadSHA,
		Ref:       &input.Ref,
		Sarif:     &encoded,
	})
	if err != nil {
		return fmt.Errorf("upload SARIF: %w", err)
	}
	return nil
}
