package auth

import (
	"testing"
	"time"
)

func TestIssueAndValidateAccessToken(t *testing.T) {
	m := TokenManager{Secret: []byte("secret"), Issuer: "docsapp"}
	tok, err := m.IssueAccessToken("u1", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := m.ValidateAccessToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Sub != "u1" {
		t.Fatalf("unexpected sub: %s", claims.Sub)
	}
}

func TestIssueRefreshToken(t *testing.T) {
	m := TokenManager{Secret: []byte("secret"), Issuer: "docsapp"}
	tok, err := m.IssueRefreshToken("u1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := m.ValidateAccessToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Sub != "u1" {
		t.Fatalf("unexpected sub: %s", claims.Sub)
	}
}
