package main

import (
	"log"
	"os"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/AndreKurait/TemporalCI/internal/activities"
	"github.com/AndreKurait/TemporalCI/internal/config"
	"github.com/AndreKurait/TemporalCI/internal/workflows"
)

const taskQueue = "temporalci-task-queue"

func main() {
	cfg := config.LoadConfig()

	c, err := client.Dial(client.Options{HostPort: cfg.TemporalHostPort})
	if err != nil {
		log.Fatalf("Unable to create Temporal client: %v", err)
	}
	defer c.Close()

	w := worker.New(c, taskQueue, worker.Options{})

	w.RegisterWorkflow(workflows.CIPipeline)
	w.RegisterWorkflow(workflows.ClusterPool)
	w.RegisterWorkflow(workflows.HelmTestPipeline)

	acts := &activities.Activities{
		GitHubToken:    cfg.GitHubToken,
		TemporalWebURL: cfg.TemporalWebURL,
		AWSRegion:      cfg.AWSRegion,
	}

	// Create in-cluster K8s client if running in K8s
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		restCfg, err := rest.InClusterConfig()
		if err == nil {
			k8sClient, err := kubernetes.NewForConfig(restCfg)
			if err == nil {
				acts.K8sClient = k8sClient
				log.Println("K8s client initialized (in-cluster mode)")
			}
		}
	}

	w.RegisterActivity(acts)

	log.Printf("Starting worker on task queue %q, Temporal at %s", taskQueue, cfg.TemporalHostPort)
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}
}
