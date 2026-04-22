package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"diplom.com/m/internal/auth"
	"diplom.com/m/internal/ports"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	Users      ports.UserRepo
	Sessions   ports.SessionRepo
	Tokens     auth.TokenManager
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type AuthResult struct {
	AccessToken       string
	RefreshSessionID  string
	RefreshTokenPlain string
	UserID            string
}

var dummyHash []byte

func init() {
	// dummy для сравнения пустых юзеров
	dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-secret"), bcrypt.DefaultCost)
}

func (s *AuthService) Register(ctx context.Context, email, password, userAgent, ip string) (AuthResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || len(password) < 8 {
		return AuthResult{}, errors.New("invalid credentials")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AuthResult{}, err
	}
	userID, err := s.Users.Create(ctx, email, string(hash))
	if err != nil {
		return AuthResult{}, err
	}
	return s.newSession(ctx, userID, userAgent, ip)
}

func (s *AuthService) Login(ctx context.Context, email, password, userAgent, ip string) (AuthResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, err := s.Users.GetByEmail(ctx, email)
	if err != nil {
		bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return AuthResult{}, errors.New("invalid credentials")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil { // constant time сравнение
		return AuthResult{}, errors.New("invalid credentials")
	}
	return s.newSession(ctx, user.ID, userAgent, ip)
}

func (s *AuthService) Refresh(ctx context.Context, refreshCookieValue string, userAgent, ip string) (AuthResult, error) {
	sessionID, plain, err := auth.ParseRefreshCookie(refreshCookieValue)
	if err != nil {
		return AuthResult{}, err
	}
	session, err := s.Sessions.GetValid(ctx, sessionID)
	if err != nil {
		return AuthResult{}, errors.New("invalid refresh session")
	}
	if session.ExpiresAt.Before(time.Now().UTC()) {
		_ = s.Sessions.Revoke(ctx, session.ID)
		return AuthResult{}, errors.New("refresh session expired")
	}
	if auth.HashRefreshToken(plain) != session.RefreshHash {
		_ = s.Sessions.Revoke(ctx, session.ID)
		return AuthResult{}, errors.New("invalid refresh token")
	}
	_ = s.Sessions.Revoke(ctx, session.ID)
	return s.newSession(ctx, session.UserID, userAgent, ip)
}

func (s *AuthService) Logout(ctx context.Context, refreshCookieValue string) error {
	sessionID, _, err := auth.ParseRefreshCookie(refreshCookieValue)
	if err != nil {
		return nil
	}
	return s.Sessions.Revoke(ctx, sessionID)
}

func (s *AuthService) ValidateAccessToken(token string) (string, error) {
	claims, err := s.Tokens.ValidateAccessToken(token)
	if err != nil {
		return "", err
	}
	return claims.Sub, nil
}

func (s *AuthService) newSession(ctx context.Context, userID, userAgent, ip string) (AuthResult, error) {
	access, err := s.Tokens.IssueAccessToken(userID, s.AccessTTL)
	if err != nil {
		return AuthResult{}, err
	}
	plain, hash, err := auth.NewRefreshToken()
	if err != nil {
		return AuthResult{}, err
	}
	sid, err := s.Sessions.Create(ctx, userID, hash, time.Now().UTC().Add(s.RefreshTTL), userAgent, ip)
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{
		AccessToken:       access,
		RefreshSessionID:  sid,
		RefreshTokenPlain: plain,
		UserID:            userID,
	}, nil
}
