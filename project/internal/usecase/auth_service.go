package usecase

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"diplom.com/m/internal/auth"
	"diplom.com/m/internal/ports"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type AuthService struct {
	Users      ports.UserRepo
	Tokens     auth.TokenManager
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type AuthResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	TokenType    string
	UserID       int64
}

var dummyHash []byte

func init() {
	dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-secret"), bcrypt.DefaultCost)
}

func (s *AuthService) Register(ctx context.Context, email, password, fullName string) (AuthResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	fullName = strings.TrimSpace(fullName)
	if email == "" || len(password) < 8 || fullName == "" {
		return AuthResult{}, ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AuthResult{}, err
	}
	userID, err := s.Users.Create(ctx, email, fullName, string(hash))
	if err != nil {
		return AuthResult{}, err
	}
	return s.issue(userID)
}

func (s *AuthService) Token(ctx context.Context, username, password string) (AuthResult, error) {
	email := strings.TrimSpace(strings.ToLower(username))
	user, err := s.Users.GetByEmail(ctx, email)
	if err != nil {
		// Compare against a dummy hash to keep timing constant for unknown users.
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return AuthResult{}, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return AuthResult{}, ErrInvalidCredentials
	}
	return s.issue(user.ID)
}

func (s *AuthService) ValidateAccessToken(token string) (int64, error) {
	claims, err := s.Tokens.ValidateAccessToken(token)
	if err != nil {
		return 0, err
	}
	id, err := strconv.ParseInt(claims.Sub, 10, 64)
	if err != nil {
		return 0, errors.New("invalid subject")
	}
	return id, nil
}

func (s *AuthService) issue(userID int64) (AuthResult, error) {
	sub := strconv.FormatInt(userID, 10)
	access, err := s.Tokens.IssueAccessToken(sub, s.AccessTTL)
	if err != nil {
		return AuthResult{}, err
	}
	refresh, err := s.Tokens.IssueRefreshToken(sub, s.RefreshTTL)
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int(s.AccessTTL.Seconds()),
		TokenType:    "Bearer",
		UserID:       userID,
	}, nil
}
