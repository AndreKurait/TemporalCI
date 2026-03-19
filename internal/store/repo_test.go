package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRepoStore_RegisterAndGet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repos.json")
	s, err := NewRepoStore(path)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	repo := &Repo{FullName: "owner/repo", DefaultBranch: "main"}
	if err := s.Register(ctx, repo); err != nil {
		t.Fatal(err)
	}

	got, ok := s.Get(ctx, "owner/repo")
	if !ok {
		t.Fatal("repo not found")
	}
	if got.DefaultBranch != "main" {
		t.Errorf("branch = %q, want main", got.DefaultBranch)
	}
	if got.CreatedAt.IsZero() {
		t.Error("createdAt should be set")
	}
}

func TestRepoStore_List(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repos.json")
	s, _ := NewRepoStore(path)
	ctx := context.Background()

	s.Register(ctx, &Repo{FullName: "a/b"})
	s.Register(ctx, &Repo{FullName: "c/d"})

	repos := s.List(ctx)
	if len(repos) != 2 {
		t.Errorf("list = %d repos, want 2", len(repos))
	}
}

func TestRepoStore_Delete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repos.json")
	s, _ := NewRepoStore(path)
	ctx := context.Background()

	s.Register(ctx, &Repo{FullName: "a/b"})
	s.Delete(ctx, "a/b")

	_, ok := s.Get(ctx, "a/b")
	if ok {
		t.Error("repo should be deleted")
	}
}

func TestRepoStore_Update(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repos.json")
	s, _ := NewRepoStore(path)
	ctx := context.Background()

	s.Register(ctx, &Repo{FullName: "a/b", DefaultBranch: "main"})
	s.Register(ctx, &Repo{FullName: "a/b", DefaultBranch: "develop"})

	got, _ := s.Get(ctx, "a/b")
	if got.DefaultBranch != "develop" {
		t.Errorf("branch = %q, want develop", got.DefaultBranch)
	}
	if len(s.List(ctx)) != 1 {
		t.Error("should not duplicate on update")
	}
}

func TestRepoStore_Persistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repos.json")
	s1, _ := NewRepoStore(path)
	s1.Register(context.Background(), &Repo{FullName: "a/b", DefaultBranch: "main"})

	// Reload from disk
	s2, err := NewRepoStore(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s2.Get(context.Background(), "a/b")
	if !ok || got.DefaultBranch != "main" {
		t.Error("repo should persist across reloads")
	}
}
