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
	"os"
)

func main() {
	http.HandleFunc("/webhook", handleWebhook)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting webhook server on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("server error: %v", err)
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

	secret := os.Getenv("GITHUB_WEBHOOK_SECRET")
	if secret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if !verifySignature(body, sig, secret) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	event := r.Header.Get("X-GitHub-Event")
	log.Printf("Received event: %s", event)

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// TODO: Start Temporal workflow here
	log.Printf("Would start CI pipeline for event: %s", event)
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, `{"status": "accepted", "event": %q}`, event)
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
