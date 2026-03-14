package main

import (
	"fmt"
	"os"
)

func main() {
	mode := "worker"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	fmt.Printf("TemporalCI %s starting...\n", mode)
	fmt.Println("Temporal SDK integration pending - see internal/ for workflow definitions")
}
