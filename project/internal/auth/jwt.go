package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Claims struct {
	Sub string `json:"sub"`
	Exp int64  `json:"exp"`
	Iat int64  `json:"iat"`
	Iss string `json:"iss"`
}

type TokenManager struct {
	Secret []byte
	Issuer string
}

func (m TokenManager) IssueAccessToken(userID string, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		Sub: userID,
		Exp: now.Add(ttl).Unix(),
		Iat: now.Unix(),
		Iss: m.Issuer,
	}
	return m.sign(claims)
}

// IssueRefreshToken issues a stateless, longer-lived token. v0 has no refresh
// endpoint, so it is not validated server-side yet; it satisfies the documented
// TokenResponse shape and is ready for a future /auth/refresh flow.
func (m TokenManager) IssueRefreshToken(userID string, ttl time.Duration) (string, error) {
	return m.IssueAccessToken(userID, ttl)
}

func (m TokenManager) ValidateAccessToken(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("invalid token format")
	}
	signed := parts[0] + "." + parts[1]
	expected := m.hmac(signed)
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return Claims{}, errors.New("invalid token signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, err
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, err
	}
	if claims.Iss != m.Issuer {
		return Claims{}, errors.New("invalid issuer")
	}
	if time.Now().UTC().Unix() >= claims.Exp {
		return Claims{}, errors.New("token expired")
	}
	if claims.Sub == "" {
		return Claims{}, errors.New("missing subject")
	}
	return claims, nil
}

func (m TokenManager) sign(claims Claims) (string, error) {
	header := map[string]any{"alg": "HS256", "typ": "JWT"}
	hb, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	pb, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	h := base64.RawURLEncoding.EncodeToString(hb)
	p := base64.RawURLEncoding.EncodeToString(pb)
	signed := h + "." + p
	sig := m.hmac(signed)
	return signed + "." + sig, nil
}

func (m TokenManager) hmac(input string) string {
	h := hmac.New(sha256.New, m.Secret)
	_, _ = h.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
