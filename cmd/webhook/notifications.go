package main

import (
	"sync"
	"time"
)

const maxNotifications = 100

// NotificationEntry represents a build notification.
type NotificationEntry struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"` // build_failed, build_recovered
	Repo       string    `json:"repo"`
	WorkflowID string    `json:"workflowId"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"createdAt"`
	Read       bool      `json:"read"`
}

// NotificationStore is a thread-safe in-memory store for recent build notifications.
type NotificationStore struct {
	mu      sync.RWMutex
	entries []NotificationEntry
}

var notificationStore = &NotificationStore{}

// AddNotification appends a notification, evicting the oldest if at capacity.
func (s *NotificationStore) AddNotification(entry NotificationEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry)
	if len(s.entries) > maxNotifications {
		s.entries = s.entries[len(s.entries)-maxNotifications:]
	}
}

// GetNotifications returns the most recent unread notifications up to limit.
func (s *NotificationStore) GetNotifications(limit int) []NotificationEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []NotificationEntry
	for i := len(s.entries) - 1; i >= 0 && len(result) < limit; i-- {
		if !s.entries[i].Read {
			result = append(result, s.entries[i])
		}
	}
	return result
}

// MarkRead marks the given notification IDs as read.
func (s *NotificationStore) MarkRead(ids []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	for i := range s.entries {
		if set[s.entries[i].ID] {
			s.entries[i].Read = true
		}
	}
}
