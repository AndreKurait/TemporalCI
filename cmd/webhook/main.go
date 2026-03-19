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
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/status", handleStatus)
	http.HandleFunc("/api/repos", middleware.AuditLog(handleRepos))
	http.HandleFunc("/api/repos/", middleware.AuditLog(handleRepoByName))
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
		fmt.Fprint(w, `{"status":"ignored","reason":"unsupported action"}`)
		return
	}

	// Enrich with repo-specific config
	if repo, ok := repoStore.Get(r.Context(), input.Repo); ok {
		input.SlackWebhookURL = repo.NotifySlack
	}
	if secretsPrefix != "" {
		input.SecretsPrefix = secretsPrefix
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
		slog.Error("failed to start workflow", "error", err)
		http.Error(w, "failed to start workflow", http.StatusInternalServerError)
		return
	}

	slog.Info("started workflow", "id", run.GetID(), "runID", run.GetRunID(), "event", event, "repo", input.Repo)
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
	// Extract repo name from path: /api/repos/owner/repo
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

func parseEvent(event string, body []byte) (workflows.CIPipelineInput, error) {
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
			return input, nil
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
