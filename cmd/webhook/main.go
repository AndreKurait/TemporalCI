package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"go.temporal.io/sdk/client"

	"github.com/AndreKurait/TemporalCI/internal/config"
	"github.com/AndreKurait/TemporalCI/internal/workflows"
)

const taskQueue = "temporalci-task-queue"

var temporalClient client.Client

func main() {
	cfg := config.LoadConfig()

	c, err := client.Dial(client.Options{HostPort: cfg.TemporalHostPort})
	if err != nil {
		log.Fatalf("Unable to create Temporal client: %v", err)
	}
	defer c.Close()
	temporalClient = c

	webhookSecret = cfg.GitHubWebhookSecret

	http.HandleFunc("/webhook", handleWebhook)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	log.Printf("Starting webhook server on :%s", cfg.WebhookPort)
	if err := http.ListenAndServe(":"+cfg.WebhookPort, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

var webhookSecret string

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	if webhookSecret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if !verifySignature(body, sig, webhookSecret) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	event := r.Header.Get("X-GitHub-Event")
	if event == "ping" {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"pong"}`)
		return
	}
	if event != "push" && event != "pull_request" {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ignored","event":%q}`, event)
		return
	}

	input, err := parseEvent(event, body)
	if err != nil {
		http.Error(w, "failed to parse event", http.StatusBadRequest)
		return
	}
	if input.Repo == "" {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ignored","reason":"unsupported action"}`)
		return
	}

	workflowID := fmt.Sprintf("ci-%s-%s-%s", input.Repo, input.Ref, event)

	// Cancel any previous run for this branch+event
	_ = temporalClient.CancelWorkflow(r.Context(), workflowID, "")

	opts := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: taskQueue,
	}

	run, err := temporalClient.ExecuteWorkflow(r.Context(), opts, "CIPipeline", input)
	if err != nil {
		log.Printf("Failed to start workflow: %v", err)
		http.Error(w, "failed to start workflow", http.StatusInternalServerError)
		return
	}

	log.Printf("Started workflow %s (run %s) for %s event", run.GetID(), run.GetRunID(), event)
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, `{"status":"accepted","workflowId":%q,"runId":%q}`, run.GetID(), run.GetRunID())
}

func parseEvent(event string, body []byte) (workflows.CIPipelineInput, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return workflows.CIPipelineInput{}, err
	}

	input := workflows.CIPipelineInput{
		Event:   event,
		Payload: string(body),
	}

	switch event {
	case "push":
		var push struct {
			Ref        string `json:"ref"`
			After      string `json:"after"`
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
		}
		if err := json.Unmarshal(body, &push); err != nil {
			return input, err
		}
		input.Repo = push.Repository.FullName
		input.Ref = push.Ref
		input.HeadSHA = push.After

	case "pull_request":
		var pr struct {
			Action      string `json:"action"`
			Number      int    `json:"number"`
			PullRequest struct {
				Head struct {
					SHA string `json:"sha"`
					Ref string `json:"ref"`
				} `json:"head"`
			} `json:"pull_request"`
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
		}
		if err := json.Unmarshal(body, &pr); err != nil {
			return input, err
		}
		if pr.Action != "opened" && pr.Action != "synchronize" {
			return input, nil // ignored action
		}
		input.Repo = pr.Repository.FullName
		input.Ref = pr.PullRequest.Head.Ref
		input.HeadSHA = pr.PullRequest.Head.SHA
		input.PRNumber = pr.Number
	}

	return input, nil
}

func verifySignature(payload []byte, signature, secret string) bool {
	if len(signature) < 7 {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
