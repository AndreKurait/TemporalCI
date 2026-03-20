package main

import (
	"fmt"
	"testing"
	"time"
)

func TestNotificationStoreAddAndGet(t *testing.T) {
	store := &NotificationStore{}
	store.AddNotification(NotificationEntry{
		ID:        "n1",
		Type:      "build_failed",
		Repo:      "owner/repo",
		Message:   "build failed",
		CreatedAt: time.Now(),
	})

	entries := store.GetNotifications(10)
	if len(entries) != 1 {
		t.Fatalf("len = %d, want 1", len(entries))
	}
	if entries[0].ID != "n1" {
		t.Errorf("ID = %q, want n1", entries[0].ID)
	}
}

func TestNotificationStoreMarkRead(t *testing.T) {
	store := &NotificationStore{}
	store.AddNotification(NotificationEntry{ID: "n1", Message: "failed", CreatedAt: time.Now()})
	store.AddNotification(NotificationEntry{ID: "n2", Message: "recovered", CreatedAt: time.Now()})

	store.MarkRead([]string{"n1"})

	entries := store.GetNotifications(10)
	if len(entries) != 1 {
		t.Fatalf("unread = %d, want 1", len(entries))
	}
	if entries[0].ID != "n2" {
		t.Errorf("remaining = %q, want n2", entries[0].ID)
	}
}

func TestNotificationStoreMaxEviction(t *testing.T) {
	store := &NotificationStore{}
	for i := 0; i < 110; i++ {
		store.AddNotification(NotificationEntry{
			ID:        fmt.Sprintf("n%d", i),
			CreatedAt: time.Now(),
		})
	}

	// GetNotifications returns unread, but we need to check total stored
	// Add one more to verify cap
	store.mu.RLock()
	count := len(store.entries)
	store.mu.RUnlock()

	if count != maxNotifications {
		t.Errorf("stored = %d, want %d", count, maxNotifications)
	}
}
