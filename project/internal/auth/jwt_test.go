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

func TestRefreshCookieRoundtrip(t *testing.T) {
	v := BuildRefreshCookieValue("sid", "token")
	sid, token, err := ParseRefreshCookie(v)
	if err != nil {
		t.Fatal(err)
	}
	if sid != "sid" || token != "token" {
		t.Fatalf("unexpected parse: %s %s", sid, token)
	}
}
