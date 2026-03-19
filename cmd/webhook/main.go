package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"go.temporal.io/sdk/client"

	"github.com/AndreKurait/TemporalCI/internal/config"
	"github.com/AndreKurait/TemporalCI/internal/middleware"
	"github.com/AndreKurait/TemporalCI/internal/store"
	"github.com/AndreKurait/TemporalCI/internal/workflows"
)

const taskQueue = "temporalci-task-queue"

var (
	temporalClient client.Client
	webhookSecret  string
	repoStore      *store.RepoStore
	secretsPrefix  string
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.LoadConfig()

	c, err := client.Dial(client.Options{HostPort: cfg.TemporalHostPort})
	if err != nil {
		log.Fatalf("Unable to create Temporal client: %v", err)
	}
	defer c.Close()
	temporalClient = c
	webhookSecret = cfg.GitHubWebhookSecret
	secretsPrefix = os.Getenv("SECRETS_PREFIX")

	// Initialize repo store
	storePath := os.Getenv("REPO_STORE_PATH")
	if storePath == "" {
		storePath = "/data/repos.json"
	}
	repoStore, err = store.NewRepoStore(storePath)
	if err != nil {
		slog.Warn("failed to init repo store, using in-memory", "error", err)
		repoStore, _ = store.NewRepoStore("/tmp/repos.json")
	}

	// Rate limiter: 60 requests per minute per IP
	limiter := middleware.NewRateLimiter(60, time.Minute)

	http.HandleFunc("/webhook", middleware.AuditLog(middleware.RateLimit(limiter, handleWebhook)))
	http.HandleFunc("/webhook/custom/", middleware.AuditLog(middleware.RateLimit(limiter, handleCustomWebhook)))
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/status", handleStatus)
	http.HandleFunc("/api/repos", middleware.AuditLog(handleRepos))
	http.HandleFunc("/api/repos/", middleware.AuditLog(handleRepoByName))
	http.HandleFunc("/api/trigger/", middleware.AuditLog(handleManualTrigger))
	http.HandleFunc("/dashboard", handleDashboard)

	slog.Info("starting webhook server", "port", cfg.WebhookPort)
	if err := http.ListenAndServe(":"+cfg.WebhookPort, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

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

	inputs, err := parseEvent(event, body)
	if err != nil {
		http.Error(w, "failed to parse event", http.StatusBadRequest)
		return
	}
	if len(inputs) == 0 {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ignored","reason":"unsupported event or action"}`)
		return
	}

	var started []map[string]string
	for _, input := range inputs {
		if input.Repo == "" {
			continue
		}

		// Enrich with repo-specific config
		if repo, ok := repoStore.Get(r.Context(), input.Repo); ok {
			input.SlackWebhookURL = repo.NotifySlack
		}
		if secretsPrefix != "" {
			input.SecretsPrefix = secretsPrefix
		}

		workflowID := fmt.Sprintf("ci-%s-%s-%s", input.Repo, input.Ref, event)
		if input.PipelineName != "" && input.PipelineName != "default" {
			workflowID = fmt.Sprintf("ci-%s-%s-%s-%s", input.Repo, input.PipelineName, input.Ref, event)
		}

		// Cancel any previous run for this branch+event+pipeline
		_ = temporalClient.CancelWorkflow(r.Context(), workflowID, "")

		opts := client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: taskQueue,
		}

		run, err := temporalClient.ExecuteWorkflow(r.Context(), opts, "CIPipeline", input)
		if err != nil {
			slog.Error("failed to start workflow", "error", err, "pipeline", input.PipelineName)
			continue
		}

		slog.Info("started workflow", "id", run.GetID(), "runID", run.GetRunID(), "event", event, "repo", input.Repo, "pipeline", input.PipelineName)
		started = append(started, map[string]string{
			"workflowId": run.GetID(), "runId": run.GetRunID(), "pipeline": input.PipelineName,
		})
	}

	if len(started) == 0 {
		http.Error(w, "failed to start any workflows", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "accepted", "workflows": started})
}

// handleCustomWebhook handles POST /webhook/custom/{owner}/{repo} for non-GitHub webhooks.
func handleCustomWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract repo from path: /webhook/custom/owner/repo
	path := strings.TrimPrefix(r.URL.Path, "/webhook/custom/")
	if path == "" {
		http.Error(w, "repo required", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Extract parameters if provided
	params := map[string]string{}
	if p, ok := payload["parameters"].(map[string]interface{}); ok {
		for k, v := range p {
			params[k] = fmt.Sprintf("%v", v)
		}
	}

	repo, ok := repoStore.Get(r.Context(), path)
	if !ok {
		http.Error(w, "repo not registered", http.StatusNotFound)
		return
	}

	input := workflows.CIPipelineInput{
		Event:      "webhook",
		Payload:    string(body),
		Repo:       repo.FullName,
		Ref:        repo.DefaultBranch,
		Parameters: params,
	}
	if secretsPrefix != "" {
		input.SecretsPrefix = secretsPrefix
	}

	workflowID := fmt.Sprintf("ci-%s-webhook-%d", repo.FullName, time.Now().UnixMilli())
	opts := client.StartWorkflowOptions{ID: workflowID, TaskQueue: taskQueue}

	run, err := temporalClient.ExecuteWorkflow(r.Context(), opts, "CIPipeline", input)
	if err != nil {
		http.Error(w, "failed to start workflow", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "accepted", "workflowId": run.GetID(), "runId": run.GetRunID(),
	})
}

// handleManualTrigger handles POST /api/trigger/{owner}/{repo}[/{pipeline}]
func handleManualTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/trigger/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 {
		http.Error(w, "repo required: /api/trigger/{owner}/{repo}", http.StatusBadRequest)
		return
	}

	repoName := parts[0] + "/" + parts[1]
	pipelineName := ""
	if len(parts) == 3 {
		pipelineName = parts[2]
	}

	var body struct {
		Ref        string            `json:"ref"`
		Parameters map[string]string `json:"parameters"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	repo, ok := repoStore.Get(r.Context(), repoName)
	if !ok {
		http.Error(w, "repo not registered", http.StatusNotFound)
		return
	}

	ref := body.Ref
	if ref == "" {
		ref = repo.DefaultBranch
	}

	input := workflows.CIPipelineInput{
		Event:        "manual",
		Repo:         repo.FullName,
		Ref:          ref,
		PipelineName: pipelineName,
		Parameters:   body.Parameters,
	}
	if repo.NotifySlack != "" {
		input.SlackWebhookURL = repo.NotifySlack
	}
	if secretsPrefix != "" {
		input.SecretsPrefix = secretsPrefix
	}

	workflowID := fmt.Sprintf("ci-%s-manual-%d", repo.FullName, time.Now().UnixMilli())
	if pipelineName != "" {
		workflowID = fmt.Sprintf("ci-%s-%s-manual-%d", repo.FullName, pipelineName, time.Now().UnixMilli())
	}

	opts := client.StartWorkflowOptions{ID: workflowID, TaskQueue: taskQueue}
	run, err := temporalClient.ExecuteWorkflow(r.Context(), opts, "CIPipeline", input)
	if err != nil {
		http.Error(w, "failed to start workflow", http.StatusInternalServerError)
		return
	}

	slog.Info("manual trigger", "repo", repo.FullName, "pipeline", pipelineName, "ref", ref)
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "accepted", "workflowId": run.GetID(), "runId": run.GetRunID(),
	})
}

// handleRepos handles POST /api/repos (register) and GET /api/repos (list).
func handleRepos(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		repos := repoStore.List(r.Context())
		json.NewEncoder(w).Encode(repos)

	case http.MethodPost:
		var repo store.Repo
		if err := json.NewDecoder(r.Body).Decode(&repo); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		if repo.FullName == "" {
			http.Error(w, `{"error":"fullName required"}`, http.StatusBadRequest)
			return
		}
		if repo.DefaultBranch == "" {
			repo.DefaultBranch = "main"
		}
		if err := repoStore.Register(r.Context(), &repo); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		slog.Info("repo registered", "repo", repo.FullName)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(repo)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRepoByName handles GET/DELETE /api/repos/{owner}/{repo}.
func handleRepoByName(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	name := r.URL.Path[len("/api/repos/"):]
	if name == "" {
		http.Error(w, `{"error":"repo name required"}`, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		repo, ok := repoStore.Get(r.Context(), name)
		if !ok {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(repo)

	case http.MethodDelete:
		if err := repoStore.Delete(r.Context(), name); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		slog.Info("repo deleted", "repo", name)
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleDashboard serves a simple admin dashboard.
func handleDashboard(w http.ResponseWriter, r *http.Request) {
	repos := repoStore.List(r.Context())
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<!DOCTYPE html><html><head><title>TemporalCI</title>
<style>body{font-family:system-ui;max-width:800px;margin:40px auto;padding:0 20px}
table{width:100%;border-collapse:collapse}th,td{text-align:left;padding:8px;border-bottom:1px solid #ddd}
h1{color:#333}.badge{display:inline-block;padding:2px 8px;border-radius:4px;font-size:12px;background:#e1f5fe}</style>
</head><body><h1>TemporalCI Dashboard</h1>`)
	fmt.Fprintf(w, `<p>%d registered repos</p>`, len(repos))
	fmt.Fprint(w, `<table><tr><th>Repository</th><th>Branch</th><th>Slack</th><th>Registered</th></tr>`)
	for _, repo := range repos {
		slack := "—"
		if repo.NotifySlack != "" {
			slack = "✅"
		}
		fmt.Fprintf(w, `<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			repo.FullName, repo.DefaultBranch, slack, repo.CreatedAt.Format("2006-01-02"))
	}
	fmt.Fprint(w, `</table></body></html>`)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"service": "TemporalCI", "status": "healthy"})
}

// parseEvent parses a GitHub webhook event and returns one or more pipeline inputs.
// Multiple inputs are returned when a push/PR could trigger multiple named pipelines.
func parseEvent(event string, body []byte) ([]workflows.CIPipelineInput, error) {
	switch event {
	case "push":
		return parsePushEvent(body)
	case "pull_request":
		return parsePREvent(body)
	case "release":
		return parseReleaseEvent(body)
	case "issues":
		return parseIssuesEvent(body)
	default:
		return nil, nil
	}
}

func parsePushEvent(body []byte) ([]workflows.CIPipelineInput, error) {
	var push struct {
		Ref        string `json:"ref"`
		After      string `json:"after"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &push); err != nil {
		return nil, err
	}

	input := workflows.CIPipelineInput{
		Event:   "push",
		Payload: string(body),
		Repo:    push.Repository.FullName,
		Ref:     push.Ref,
		HeadSHA: push.After,
	}

	// Detect tag push
	if strings.HasPrefix(push.Ref, "refs/tags/") {
		input.Event = "tag"
	}

	return []workflows.CIPipelineInput{input}, nil
}

func parsePREvent(body []byte) ([]workflows.CIPipelineInput, error) {
	var pr struct {
		Action      string `json:"action"`
		Number      int    `json:"number"`
		PullRequest struct {
			Head struct {
				SHA string `json:"sha"`
				Ref string `json:"ref"`
			} `json:"head"`
			Labels []struct {
				Name string `json:"name"`
			} `json:"labels"`
		} `json:"pull_request"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, err
	}
	if pr.Action != "opened" && pr.Action != "synchronize" && pr.Action != "labeled" {
		return nil, nil
	}

	var labels []string
	for _, l := range pr.PullRequest.Labels {
		labels = append(labels, l.Name)
	}

	return []workflows.CIPipelineInput{{
		Event:    "pull_request",
		Payload:  string(body),
		Repo:     pr.Repository.FullName,
		Ref:      pr.PullRequest.Head.Ref,
		HeadSHA:  pr.PullRequest.Head.SHA,
		PRNumber: pr.Number,
		Labels:   labels,
	}}, nil
}

func parseReleaseEvent(body []byte) ([]workflows.CIPipelineInput, error) {
	var rel struct {
		Action  string `json:"action"`
		Release struct {
			TagName string `json:"tag_name"`
			Name    string `json:"name"`
			HTMLURL string `json:"html_url"`
		} `json:"release"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, err
	}

	return []workflows.CIPipelineInput{{
		Event:   "release",
		Payload: string(body),
		Repo:    rel.Repository.FullName,
		Ref:     rel.Release.TagName,
		Parameters: map[string]string{
			"TEMPORALCI_TAG":          rel.Release.TagName,
			"TEMPORALCI_RELEASE_NAME": rel.Release.Name,
			"TEMPORALCI_RELEASE_URL":  rel.Release.HTMLURL,
		},
	}}, nil
}

func parseIssuesEvent(body []byte) ([]workflows.CIPipelineInput, error) {
	var issue struct {
		Action string `json:"action"`
		Issue  struct {
			Number int `json:"number"`
		} `json:"issue"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, err
	}

	return []workflows.CIPipelineInput{{
		Event:   "issues",
		Payload: string(body),
		Repo:    issue.Repository.FullName,
		Parameters: map[string]string{
			"ISSUE_ACTION": issue.Action,
			"ISSUE_NUMBER": fmt.Sprintf("%d", issue.Issue.Number),
		},
	}}, nil
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
