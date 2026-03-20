package main

import (
	"fmt"
	"testing"
	"time"
)

// --- Month 18: In-app notifications, full migration validation ---

func TestNotificationStore_OrderReversed(t *testing.T) {
	store := &NotificationStore{}
	for i := 0; i < 5; i++ {
		store.AddNotification(NotificationEntry{
			ID:        fmt.Sprintf("n%d", i),
			Message:   fmt.Sprintf("msg %d", i),
			CreatedAt: time.Now(),
		})
	}
	entries := store.GetNotifications(3)
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3", len(entries))
	}
	// Most recent first
	if entries[0].ID != "n4" {
		t.Errorf("first = %q, want n4", entries[0].ID)
	}
	if entries[2].ID != "n2" {
		t.Errorf("last = %q, want n2", entries[2].ID)
	}
}

func TestNotificationStore_MarkReadFilters(t *testing.T) {
	store := &NotificationStore{}
	store.AddNotification(NotificationEntry{ID: "a", CreatedAt: time.Now()})
	store.AddNotification(NotificationEntry{ID: "b", CreatedAt: time.Now()})
	store.AddNotification(NotificationEntry{ID: "c", CreatedAt: time.Now()})

	store.MarkRead([]string{"a", "c"})

	entries := store.GetNotifications(10)
	if len(entries) != 1 {
		t.Fatalf("unread = %d, want 1", len(entries))
	}
	if entries[0].ID != "b" {
		t.Errorf("remaining = %q, want b", entries[0].ID)
	}
}

func TestNotificationStore_Types(t *testing.T) {
	store := &NotificationStore{}
	store.AddNotification(NotificationEntry{
		ID:         "n1",
		Type:       "build_failed",
		Repo:       "owner/repo",
		WorkflowID: "ci-owner/repo-main-push",
		Message:    "Build failed: gradle-tests [index=7]",
		CreatedAt:  time.Now(),
	})
	store.AddNotification(NotificationEntry{
		ID:         "n2",
		Type:       "build_recovered",
		Repo:       "owner/repo",
		WorkflowID: "ci-owner/repo-main-push",
		Message:    "Build recovered on main",
		CreatedAt:  time.Now(),
	})

	entries := store.GetNotifications(10)
	if len(entries) != 2 {
		t.Fatalf("len = %d, want 2", len(entries))
	}
	if entries[0].Type != "build_recovered" {
		t.Errorf("type = %q", entries[0].Type)
	}
}

func TestNotificationStore_EmptyGetNotifications(t *testing.T) {
	store := &NotificationStore{}
	entries := store.GetNotifications(10)
	if entries != nil && len(entries) != 0 {
		t.Errorf("expected empty, got %v", entries)
	}
}

func TestNotificationStore_AllRead(t *testing.T) {
	store := &NotificationStore{}
	store.AddNotification(NotificationEntry{ID: "x", CreatedAt: time.Now()})
	store.MarkRead([]string{"x"})

	entries := store.GetNotifications(10)
	if len(entries) != 0 {
		t.Errorf("expected 0 unread, got %d", len(entries))
	}
}

func TestNotificationStore_ConcurrentSafety(t *testing.T) {
	store := &NotificationStore{}
	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 50; i++ {
			store.AddNotification(NotificationEntry{
				ID:        fmt.Sprintf("w%d", i),
				CreatedAt: time.Now(),
			})
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 50; i++ {
			store.GetNotifications(10)
		}
		done <- true
	}()

	<-done
	<-done
	// No race condition = pass
}
