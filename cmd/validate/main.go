package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/AndreKurait/TemporalCI/internal/config"
)

func main() {
	dir := "."
	dryRun := false

	for _, arg := range os.Args[1:] {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "-h", "--help":
			fmt.Println("Usage: temporalci-validate [--dry-run] [directory]")
			fmt.Println("  Validates .temporalci/ pipelines in the given directory (default: current)")
			os.Exit(0)
		default:
			dir = arg
		}
	}

	cfg, err := config.LoadPipelineConfig(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	errs := cfg.Validate()
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "Validation errors:\n")
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		os.Exit(1)
	}

	if dryRun {
		pipelines := cfg.GetPipelines()
		out, _ := json.MarshalIndent(pipelines, "", "  ")
		fmt.Println(string(out))
		return
	}

	fmt.Println("✅ .temporalci/ pipelines are valid")
	pipelines := cfg.GetPipelines()
	for name, p := range pipelines {
		fmt.Printf("  Pipeline %q: %d steps", name, len(p.Steps))
		if p.Post != nil {
			fmt.Printf(", %d post steps", len(p.Post.Always)+len(p.Post.OnFailure))
		}
		if len(p.Parameters) > 0 {
			fmt.Printf(", %d parameters", len(p.Parameters))
		}
		fmt.Println()
	}
}
