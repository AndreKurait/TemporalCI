package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/AndreKurait/TemporalCI/internal/activities"
	"github.com/AndreKurait/TemporalCI/internal/config"
	"github.com/AndreKurait/TemporalCI/internal/ghapp"
	"github.com/AndreKurait/TemporalCI/internal/workflows"
)

const taskQueue = "temporalci-task-queue"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.LoadConfig()

	// Prometheus metrics endpoint
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		slog.Info("metrics server starting", "port", "9090")
		if err := http.ListenAndServe(":9090", nil); err != nil {
			slog.Error("metrics server failed", "error", err)
		}
	}()

	c, err := client.Dial(client.Options{HostPort: cfg.TemporalHostPort})
	if err != nil {
		log.Fatalf("Unable to create Temporal client: %v", err)
	}
	defer c.Close()

	w := worker.New(c, taskQueue, worker.Options{})
	w.RegisterWorkflow(workflows.CIPipeline)
	w.RegisterWorkflow(workflows.MatrixChild)
	w.RegisterWorkflow(workflows.PodCleanup)
	w.RegisterWorkflow(workflows.ApprovalGate)
	w.RegisterWorkflow(workflows.ClusterPool)
	w.RegisterWorkflow(workflows.HelmTestPipeline)

	acts := &activities.Activities{
		GitHubToken:    cfg.GitHubToken,
		TemporalWebURL: cfg.TemporalWebURL,
		LogBucket:      cfg.LogBucket,
		CINodePool:     os.Getenv("CI_NODE_POOL") == "true",
		SecretsPrefix:  os.Getenv("SECRETS_PREFIX"),
	}

	// GitHub App authentication (preferred over PAT)
	if appIDStr := os.Getenv("GITHUB_APP_ID"); appIDStr != "" {
		appID, _ := strconv.ParseInt(appIDStr, 10, 64)
		pemKey, err := os.ReadFile(os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH"))
		if err != nil {
			// Try env var directly
			pemKey = []byte(os.Getenv("GITHUB_APP_PRIVATE_KEY"))
		}
		if len(pemKey) > 0 && appID > 0 {
			appClient, err := ghapp.New(appID, pemKey)
			if err != nil {
				slog.Warn("failed to init GitHub App", "error", err)
			} else {
				acts.GitHubApp = appClient
				slog.Info("GitHub App initialized", "appID", appID)
			}
		}
	}

	// In-cluster K8s client
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		if restCfg, err := rest.InClusterConfig(); err == nil {
			if k8sClient, err := kubernetes.NewForConfig(restCfg); err == nil {
				acts.K8sClient = k8sClient
				slog.Info("K8s client initialized (in-cluster mode)")
			}
		}
	}

	// AWS clients
	awsCfg, awsErr := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.AWSRegion),
	)
	if awsErr == nil {
		// S3 client for log uploads
		if cfg.LogBucket != "" {
			s3Client := s3.NewFromConfig(awsCfg)
			acts.S3Client = s3Client
			acts.S3Presigner = s3.NewPresignClient(s3Client)
			slog.Info("S3 client initialized", "bucket", cfg.LogBucket)
		}

		// Secrets Manager client
		acts.SecretsClient = secretsmanager.NewFromConfig(awsCfg)
		slog.Info("Secrets Manager client initialized")
	} else {
		slog.Warn("failed to init AWS config", "error", awsErr)
	}

	w.RegisterActivity(acts)

	go scheduleCleanup(c)

	slog.Info("starting worker", "taskQueue", taskQueue)
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}
}

func scheduleCleanup(c client.Client) {
	time.Sleep(10 * time.Second)
	handle, err := c.ScheduleClient().Create(context.Background(), client.ScheduleOptions{
		ID: "pod-cleanup",
		Spec: client.ScheduleSpec{
			Intervals: []client.ScheduleIntervalSpec{{Every: 1 * time.Hour}},
		},
		Action: &client.ScheduleWorkflowAction{
			ID: "pod-cleanup", Workflow: workflows.PodCleanup, TaskQueue: taskQueue,
		},
	})
	if err != nil {
		slog.Info("schedule create result", "error", err)
		return
	}
	slog.Info("pod cleanup schedule created", "id", handle.GetID())
}
