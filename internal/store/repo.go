package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Repo represents a registered repository.
type Repo struct {
	FullName       string    `json:"fullName"`       // e.g. "owner/repo"
	DefaultBranch  string    `json:"defaultBranch"`  // e.g. "main"
	InstallationID int64     `json:"installationID"` // GitHub App installation ID
	WebhookID      int64     `json:"webhookID"`      // GitHub webhook ID (if created by us)
	NotifySlack    string    `json:"notifySlack,omitempty"` // Slack webhook URL
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// RepoStore persists repo registrations. Uses a JSON file for simplicity;
// swap to PostgreSQL when needed.
type RepoStore struct {
	mu   sync.RWMutex
	path string
	data map[string]*Repo // keyed by fullName
}

// NewRepoStore creates or loads a repo store from disk.
func NewRepoStore(path string) (*RepoStore, error) {
	s := &RepoStore{path: path, data: make(map[string]*Repo)}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &s.data); err != nil {
			return nil, fmt.Errorf("parse repo store: %w", err)
		}
	}
	return s, nil
}

// Register adds or updates a repo registration.
func (s *RepoStore) Register(_ context.Context, repo *Repo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if existing, ok := s.data[repo.FullName]; ok {
		existing.DefaultBranch = repo.DefaultBranch
		existing.InstallationID = repo.InstallationID
		existing.NotifySlack = repo.NotifySlack
		existing.UpdatedAt = now
	} else {
		repo.CreatedAt = now
		repo.UpdatedAt = now
		s.data[repo.FullName] = repo
	}
	return s.save()
}

// Get returns a repo by full name.
func (s *RepoStore) Get(_ context.Context, fullName string) (*Repo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.data[fullName]
	return r, ok
}

// List returns all registered repos.
func (s *RepoStore) List(_ context.Context) []*Repo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	repos := make([]*Repo, 0, len(s.data))
	for _, r := range s.data {
		repos = append(repos, r)
	}
	return repos
}

// Delete removes a repo registration.
func (s *RepoStore) Delete(_ context.Context, fullName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, fullName)
	return s.save()
}

func (s *RepoStore) save() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0644)
}
