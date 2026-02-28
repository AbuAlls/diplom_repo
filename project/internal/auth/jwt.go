package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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

func NewRefreshToken() (plain string, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	plain = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(plain))
	hash = hex.EncodeToString(sum[:])
	return plain, hash, nil
}

func HashRefreshToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func ParseRefreshCookie(raw string) (sessionID string, token string, err error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("invalid refresh token")
	}
	return parts[0], parts[1], nil
}

func BuildRefreshCookieValue(sessionID, token string) string {
	return fmt.Sprintf("%s.%s", sessionID, token)
}
