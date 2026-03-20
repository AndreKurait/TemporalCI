package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/workflowservice/v1"

	"github.com/AndreKurait/TemporalCI/internal/activities"
	"github.com/AndreKurait/TemporalCI/internal/workflows"
)

type buildSummary struct {
	WorkflowID   string       `json:"workflowId"`
	RunID        string       `json:"runId"`
	Repo         string       `json:"repo"`
	Ref          string       `json:"ref"`
	Event        string       `json:"event"`
	Status       string       `json:"status"`
	StartTime    time.Time    `json:"startTime"`
	CloseTime    *time.Time   `json:"closeTime,omitempty"`
	Duration     float64      `json:"duration,omitempty"`
	PipelineName string       `json:"pipelineName,omitempty"`
	Steps        []stepBrief  `json:"steps,omitempty"`
}

type stepBrief struct {
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	Duration float64 `json:"duration"`
}

type buildDetail struct {
	WorkflowID   string                  `json:"workflowId"`
	RunID        string                  `json:"runId"`
	Repo         string                  `json:"repo"`
	Ref          string                  `json:"ref"`
	Event        string                  `json:"event"`
	HeadSHA      string                  `json:"headSHA,omitempty"`
	PRNumber     int                     `json:"prNumber,omitempty"`
	Status       string                  `json:"status"`
	StartTime    time.Time               `json:"startTime"`
	CloseTime    *time.Time              `json:"closeTime,omitempty"`
	Duration     float64                 `json:"duration,omitempty"`
	PipelineName string                  `json:"pipelineName,omitempty"`
	Steps        []activities.StepResult `json:"steps,omitempty"`
	Parameters   map[string]string       `json:"parameters,omitempty"`
}

type repoStatus struct {
	Repo          string         `json:"repo"`
	DefaultBranch string         `json:"defaultBranch"`
	LatestBuild   *buildSummary  `json:"latestBuild,omitempty"`
	RecentBuilds  []string       `json:"recentBuilds"`
	Pipelines     []string       `json:"pipelines,omitempty"`
}

type analyticsResult struct {
	SuccessRate  float64         `json:"successRate"`
	AvgDuration  float64         `json:"avgDuration"`
	BuildCount   int             `json:"buildCount"`
	FailingSteps []stepCount     `json:"failingSteps"`
	SlowestSteps []stepDuration  `json:"slowestSteps"`
	DailyTrend   []dailyEntry   `json:"dailyTrend"`
}

type stepCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type stepDuration struct {
	Name        string  `json:"name"`
	AvgDuration float64 `json:"avgDuration"`
}

type dailyEntry struct {
	Date        string  `json:"date"`
	Count       int     `json:"count"`
	PassCount   int     `json:"passCount"`
	AvgDuration float64 `json:"avgDuration"`
}

// registerCIRoutes registers all CI API routes on the default mux.
func registerCIRoutes() {
	http.HandleFunc("/api/ci/builds", readAuth(handleCIBuilds))
	http.HandleFunc("/api/ci/builds/", readAuth(handleCIBuildDetail))
	http.HandleFunc("/api/ci/repos", readAuth(handleCIRepos))
	http.HandleFunc("/api/ci/repos/", handleCIRepoBadge)
	http.HandleFunc("/api/ci/analytics", readAuth(handleCIAnalytics))
	http.HandleFunc("/api/ci/notifications", readAuth(handleCINotifications))
	http.HandleFunc("/api/ci/notifications/read", authMiddleware(handleCINotificationsRead))

	http.HandleFunc("/auth/github", handleAuthGitHub)
	http.HandleFunc("/auth/github/callback", handleAuthCallback)
	http.HandleFunc("/auth/me", handleAuthMe)
	http.HandleFunc("/auth/logout", handleAuthLogout)
}

func handleCIBuilds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	limit := 50
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 && l <= 200 {
		limit = l
	}

	query := "WorkflowType='CIPipeline'"
	if repo := q.Get("repo"); repo != "" {
		query += fmt.Sprintf(" AND WorkflowId STARTS_WITH 'ci-%s-'", repo)
	}
	if branch := q.Get("branch"); branch != "" {
		query += fmt.Sprintf(" AND WorkflowId LIKE '%%-%s-%%'", branch)
	}
	if status := q.Get("status"); status != "" {
		if ts := ciStatusToTemporalStatus(status); ts != "" {
			query += " AND ExecutionStatus='" + ts + "'"
		}
	}
	query += " ORDER BY StartTime DESC"

	resp, err := temporalClient.ListWorkflow(r.Context(), &workflowservice.ListWorkflowExecutionsRequest{
		Namespace: "default",
		Query:     query,
		PageSize:  int32(limit),
	})
	if err != nil {
		slog.Error("list workflows failed", "error", err)
		http.Error(w, "failed to list builds", http.StatusInternalServerError)
		return
	}

	builds := make([]buildSummary, 0, len(resp.Executions))
	for _, exec := range resp.Executions {
		info := exec.GetExecution()
		start := exec.GetStartTime().AsTime()
		b := buildSummary{
			WorkflowID: info.GetWorkflowId(),
			RunID:      info.GetRunId(),
			Status:     mapWorkflowStatus(exec.GetStatus()),
			StartTime:  start,
		}
		if ct := exec.GetCloseTime(); ct != nil && !ct.AsTime().IsZero() {
			t := ct.AsTime()
			b.CloseTime = &t
			b.Duration = t.Sub(start).Seconds()
		}

		input, result := extractWorkflowData(r.Context(), info.GetWorkflowId())
		if input != nil {
			b.Repo = input.Repo
			b.Ref = input.Ref
			b.Event = input.Event
			b.PipelineName = input.PipelineName
		}
		if result != nil {
			if b.Status == "completed" {
				b.Status = result.Status
			}
			b.PipelineName = result.PipelineName
			for _, s := range result.Steps {
				b.Steps = append(b.Steps, stepBrief{Name: s.Name, Status: s.Status, Duration: s.Duration})
			}
		}
		builds = append(builds, b)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(builds)
}

func handleCIBuildDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/ci/builds/")
	if path == "" {
		http.Error(w, "workflowId required", http.StatusBadRequest)
		return
	}

	// Handle /api/ci/builds/{workflowId}/steps/{stepName}/log
	if parts := strings.SplitN(path, "/steps/", 2); len(parts) == 2 {
		handleStepLog(w, r, parts[0], strings.TrimSuffix(parts[1], "/log"))
		return
	}

	workflowID := strings.TrimSuffix(path, "/")
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	desc, err := temporalClient.DescribeWorkflowExecution(r.Context(), workflowID, "")
	if err != nil {
		http.Error(w, "workflow not found", http.StatusNotFound)
		return
	}

	info := desc.GetWorkflowExecutionInfo()
	start := info.GetStartTime().AsTime()
	detail := buildDetail{
		WorkflowID: info.GetExecution().GetWorkflowId(),
		RunID:      info.GetExecution().GetRunId(),
		Status:     mapWorkflowStatus(info.GetStatus()),
		StartTime:  start,
	}
	if ct := info.GetCloseTime(); ct != nil && !ct.AsTime().IsZero() {
		t := ct.AsTime()
		detail.CloseTime = &t
		detail.Duration = t.Sub(start).Seconds()
	}

	input, result := extractWorkflowData(r.Context(), workflowID)
	if input != nil {
		detail.Repo = input.Repo
		detail.Ref = input.Ref
		detail.Event = input.Event
		detail.HeadSHA = input.HeadSHA
		detail.PRNumber = input.PRNumber
		detail.PipelineName = input.PipelineName
		detail.Parameters = input.Parameters
	}
	if result != nil {
		if detail.Status == "completed" {
			detail.Status = result.Status
		}
		detail.Steps = result.Steps
		if result.PipelineName != "" {
			detail.PipelineName = result.PipelineName
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detail)
}

func handleStepLog(w http.ResponseWriter, r *http.Request, workflowID, stepName string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_, result := extractWorkflowData(r.Context(), workflowID)
	if result == nil {
		http.Error(w, "workflow result not found", http.StatusNotFound)
		return
	}

	for _, s := range result.Steps {
		if s.Name == stepName && s.LogURL != "" {
			http.Redirect(w, r, s.LogURL, http.StatusTemporaryRedirect)
			return
		}
	}
	http.Error(w, "step log not found", http.StatusNotFound)
}

func handleCIRepos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	repos := repoStore.List(r.Context())
	result := make([]repoStatus, 0, len(repos))

	for _, repo := range repos {
		rs := repoStatus{
			Repo:          repo.FullName,
			DefaultBranch: repo.DefaultBranch,
			RecentBuilds:  []string{},
		}

		query := fmt.Sprintf("WorkflowType='CIPipeline' AND WorkflowId STARTS_WITH 'ci-%s-' ORDER BY StartTime DESC", repo.FullName)
		resp, err := temporalClient.ListWorkflow(r.Context(), &workflowservice.ListWorkflowExecutionsRequest{
			Namespace: "default",
			Query:     query,
			PageSize:  10,
		})
		if err == nil && len(resp.Executions) > 0 {
			first := resp.Executions[0]
			info := first.GetExecution()
			start := first.GetStartTime().AsTime()
			latest := &buildSummary{
				WorkflowID: info.GetWorkflowId(),
				RunID:      info.GetRunId(),
				Status:     mapWorkflowStatus(first.GetStatus()),
				StartTime:  start,
			}
			if ct := first.GetCloseTime(); ct != nil && !ct.AsTime().IsZero() {
				t := ct.AsTime()
				latest.CloseTime = &t
				latest.Duration = t.Sub(start).Seconds()
			}
			input, res := extractWorkflowData(r.Context(), info.GetWorkflowId())
			if input != nil {
				latest.Repo = input.Repo
				latest.Ref = input.Ref
				latest.Event = input.Event
			}
			if res != nil && latest.Status == "completed" {
				latest.Status = res.Status
			}
			rs.LatestBuild = latest

			pipelineSet := map[string]bool{}
			for _, exec := range resp.Executions {
				st := mapWorkflowStatus(exec.GetStatus())
				if _, res := extractWorkflowData(r.Context(), exec.GetExecution().GetWorkflowId()); res != nil {
					if st == "completed" {
						st = res.Status
					}
					if res.PipelineName != "" {
						pipelineSet[res.PipelineName] = true
					}
				}
				rs.RecentBuilds = append(rs.RecentBuilds, st)
			}
			for p := range pipelineSet {
				rs.Pipelines = append(rs.Pipelines, p)
			}
		}
		result = append(result, rs)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleCIRepoBadge(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/ci/repos/")
	if !strings.HasSuffix(path, "/badge.svg") {
		// Not a badge request — fall through to 404
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	path = strings.TrimSuffix(path, "/badge.svg")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		http.Error(w, "path: /api/ci/repos/{owner}/{repo}/badge.svg", http.StatusBadRequest)
		return
	}
	repoName := parts[0] + "/" + parts[1]

	repo, ok := repoStore.Get(r.Context(), repoName)
	if !ok {
		http.Error(w, "repo not found", http.StatusNotFound)
		return
	}

	status := "unknown"
	query := fmt.Sprintf("WorkflowType='CIPipeline' AND WorkflowId STARTS_WITH 'ci-%s-refs/heads/%s-' ORDER BY StartTime DESC", repoName, repo.DefaultBranch)
	resp, err := temporalClient.ListWorkflow(r.Context(), &workflowservice.ListWorkflowExecutionsRequest{
		Namespace: "default",
		Query:     query,
		PageSize:  1,
	})
	if err == nil && len(resp.Executions) > 0 {
		status = mapWorkflowStatus(resp.Executions[0].GetStatus())
		if status == "completed" {
			if _, res := extractWorkflowData(r.Context(), resp.Executions[0].GetExecution().GetWorkflowId()); res != nil {
				status = res.Status
			}
		}
	}

	color := "#9f9f9f"
	label := status
	switch status {
	case "passed":
		label = "passing"
		color = "#4c1"
	case "failed":
		label = "failing"
		color = "#e05d44"
	case "running":
		color = "#dfb317"
	case "cancelled":
		color = "#9f9f9f"
	}

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	fmt.Fprint(w, badgeSVG("build", label, color))
}

func handleCIAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	repo := r.URL.Query().Get("repo")
	if repo == "" {
		http.Error(w, "repo query param required", http.StatusBadRequest)
		return
	}

	days := 30
	if d, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && d > 0 && d <= 365 {
		days = d
	}

	since := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)
	query := fmt.Sprintf("WorkflowType='CIPipeline' AND WorkflowId STARTS_WITH 'ci-%s-' AND StartTime > '%s' ORDER BY StartTime DESC", repo, since)

	resp, err := temporalClient.ListWorkflow(r.Context(), &workflowservice.ListWorkflowExecutionsRequest{
		Namespace: "default",
		Query:     query,
		PageSize:  200,
	})
	if err != nil {
		http.Error(w, "failed to query workflows", http.StatusInternalServerError)
		return
	}

	var (
		totalDuration float64
		passCount     int
		failCounts    = map[string]int{}
		stepDurations = map[string][]float64{}
		daily         = map[string]*dailyEntry{}
	)

	for _, exec := range resp.Executions {
		start := exec.GetStartTime().AsTime()
		dateKey := start.Format("2006-01-02")
		if daily[dateKey] == nil {
			daily[dateKey] = &dailyEntry{Date: dateKey}
		}
		daily[dateKey].Count++

		var dur float64
		if ct := exec.GetCloseTime(); ct != nil && !ct.AsTime().IsZero() {
			dur = ct.AsTime().Sub(start).Seconds()
		}
		totalDuration += dur
		daily[dateKey].AvgDuration += dur

		status := mapWorkflowStatus(exec.GetStatus())
		_, result := extractWorkflowData(r.Context(), exec.GetExecution().GetWorkflowId())
		if result != nil && status == "completed" {
			status = result.Status
		}

		if status == "passed" {
			passCount++
			daily[dateKey].PassCount++
		}

		if result != nil {
			for _, s := range result.Steps {
				if s.Status == "failed" {
					failCounts[s.Name]++
				}
				stepDurations[s.Name] = append(stepDurations[s.Name], s.Duration)
			}
		}
	}

	total := len(resp.Executions)
	analytics := analyticsResult{BuildCount: total}
	if total > 0 {
		analytics.SuccessRate = float64(passCount) / float64(total) * 100
		analytics.AvgDuration = totalDuration / float64(total)
	}

	analytics.FailingSteps = topNStepCounts(failCounts, 5)
	analytics.SlowestSteps = topNStepDurations(stepDurations, 5)

	trend := make([]dailyEntry, 0, len(daily))
	for _, d := range daily {
		if d.Count > 0 {
			d.AvgDuration /= float64(d.Count)
		}
		trend = append(trend, *d)
	}
	sort.Slice(trend, func(i, j int) bool { return trend[i].Date < trend[j].Date })
	analytics.DailyTrend = trend

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analytics)
}

func handleCINotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 20
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}
	entries := notificationStore.GetNotifications(limit)
	if entries == nil {
		entries = []NotificationEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func handleCINotificationsRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	notificationStore.MarkRead(body.IDs)
	w.WriteHeader(http.StatusNoContent)
}

// extractWorkflowData reads workflow history to extract input and result.
func extractWorkflowData(ctx context.Context, workflowID string) (*workflows.CIPipelineInput, *workflows.CIPipelineResult) {
	iter := temporalClient.GetWorkflowHistory(ctx, workflowID, "", false, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	var input *workflows.CIPipelineInput
	var result *workflows.CIPipelineResult

	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			break
		}
		if attrs := event.GetWorkflowExecutionStartedEventAttributes(); attrs != nil {
			if attrs.Input != nil && len(attrs.Input.Payloads) > 0 {
				var in workflows.CIPipelineInput
				if json.Unmarshal(attrs.Input.Payloads[0].Data, &in) == nil {
					input = &in
				}
			}
		}
		if attrs := event.GetWorkflowExecutionCompletedEventAttributes(); attrs != nil {
			if attrs.Result != nil && len(attrs.Result.Payloads) > 0 {
				var res workflows.CIPipelineResult
				if json.Unmarshal(attrs.Result.Payloads[0].Data, &res) == nil {
					result = &res
				}
			}
		}
	}
	return input, result
}

func mapWorkflowStatus(s enums.WorkflowExecutionStatus) string {
	switch s {
	case enums.WORKFLOW_EXECUTION_STATUS_RUNNING:
		return "running"
	case enums.WORKFLOW_EXECUTION_STATUS_COMPLETED:
		return "completed"
	case enums.WORKFLOW_EXECUTION_STATUS_FAILED:
		return "failed"
	case enums.WORKFLOW_EXECUTION_STATUS_CANCELED:
		return "cancelled"
	case enums.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:
		return "timed_out"
	default:
		return "unknown"
	}
}

func ciStatusToTemporalStatus(s string) string {
	switch s {
	case "running":
		return "Running"
	case "completed", "passed", "failed":
		return "Completed"
	case "cancelled":
		return "Canceled"
	case "timed_out":
		return "TimedOut"
	default:
		return ""
	}
}

func topNStepCounts(counts map[string]int, n int) []stepCount {
	result := make([]stepCount, 0, len(counts))
	for name, count := range counts {
		result = append(result, stepCount{Name: name, Count: count})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Count > result[j].Count })
	if len(result) > n {
		result = result[:n]
	}
	return result
}

func topNStepDurations(durations map[string][]float64, n int) []stepDuration {
	result := make([]stepDuration, 0, len(durations))
	for name, durs := range durations {
		var total float64
		for _, d := range durs {
			total += d
		}
		result = append(result, stepDuration{Name: name, AvgDuration: total / float64(len(durs))})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].AvgDuration > result[j].AvgDuration })
	if len(result) > n {
		result = result[:n]
	}
	return result
}
