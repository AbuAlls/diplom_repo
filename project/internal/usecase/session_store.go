package usecase

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// SessionStore holds short-lived, per-analyze-call nonces that map to the
// ownerID of the user who initiated the call. Callbacks from the AI agent
// present the nonce as X-Internal-Token; the handler looks it up here to
// scope subsequent queries to that owner.
type SessionStore struct {
	mu      sync.RWMutex
	entries map[string]sessionEntry
}

type sessionEntry struct {
	ownerID   int64
	expiresAt time.Time
}

func NewSessionStore() *SessionStore {
	return &SessionStore{entries: make(map[string]sessionEntry)}
}

// Create registers a new session for ownerID and returns the opaque nonce.
func (s *SessionStore) Create(ownerID int64, ttl time.Duration) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	nonce := hex.EncodeToString(b)

	s.mu.Lock()
	s.entries[nonce] = sessionEntry{ownerID: ownerID, expiresAt: time.Now().Add(ttl)}
	s.mu.Unlock()
	return nonce
}

// Lookup returns the ownerID bound to nonce if it exists and has not expired.
func (s *SessionStore) Lookup(nonce string) (int64, bool) {
	s.mu.RLock()
	e, ok := s.entries[nonce]
	s.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return 0, false
	}
	return e.ownerID, true
}

// Delete removes the nonce immediately (called in a defer after Analyze returns).
func (s *SessionStore) Delete(nonce string) {
	s.mu.Lock()
	delete(s.entries, nonce)
	s.mu.Unlock()
}
