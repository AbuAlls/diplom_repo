package usecase

import (
	"testing"
	"time"
)

func TestSessionStoreCreateLookupDelete(t *testing.T) {
	s := NewSessionStore()

	nonce := s.Create(42, time.Minute)
	if nonce == "" {
		t.Fatal("expected non-empty nonce")
	}

	ownerID, ok := s.Lookup(nonce)
	if !ok || ownerID != 42 {
		t.Fatalf("expected ownerID 42, got %d ok=%v", ownerID, ok)
	}

	// Unknown token.
	if _, ok := s.Lookup("unknown"); ok {
		t.Fatal("expected Lookup to miss for unknown token")
	}

	s.Delete(nonce)
	if _, ok := s.Lookup(nonce); ok {
		t.Fatal("expected Lookup to miss after Delete")
	}
}

func TestSessionStoreExpiry(t *testing.T) {
	s := NewSessionStore()
	nonce := s.Create(7, time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	if _, ok := s.Lookup(nonce); ok {
		t.Fatal("expected expired nonce to be rejected")
	}
}

func TestSessionStoreUniqueness(t *testing.T) {
	s := NewSessionStore()
	n1 := s.Create(1, time.Minute)
	n2 := s.Create(1, time.Minute)
	if n1 == n2 {
		t.Fatal("expected unique nonces")
	}
}
