package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidName        = errors.New("name is required")
	ErrInvalidEmail       = errors.New("invalid email")
	ErrInvalidPassword    = errors.New("password must contain 8-72 bytes")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrUserNotFound       = errors.New("user not found")
)

type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, id string) (*User, error)
	Create(ctx context.Context, user User) error
}

type Service struct {
	repo      UserRepository
	jwtSecret []byte
}

func toUserResponse(user User) *UserResponse {
	return &UserResponse{
		ID:        user.ID,
		FullName:  user.FullName,
		Email:     user.Email,
		Role:      user.Role,
		CompanyID: user.CompanyID,
	}
}
func NewService(repo UserRepository, jwtSecret string) (*Service, error) {
	if len(jwtSecret) < 32 {
		return nil, errors.New("JWT secret must be at least 32 bytes")
	}

	if repo == nil {
		return nil, errors.New("user repository is required")
	}

	return &Service{
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
	}, nil
}
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*UserResponse, error) {
	name := strings.TrimSpace(req.FullName)
	email := strings.ToLower(strings.TrimSpace(req.Email))

	if name == "" {
		return nil, ErrInvalidName
	}

	parsedEmail, err := mail.ParseAddress(email)
	if err != nil || parsedEmail.Address != email {
		return nil, ErrInvalidEmail
	}

	if len(req.Password) < 8 || len(req.Password) > 72 {
		return nil, ErrInvalidPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := User{
		ID:           uuid.NewString(),
		FullName:     name,
		Email:        email,
		PasswordHash: string(hash),
		Role:         "CLIENT",
	}
	err = s.repo.Create(ctx, user)
	if err != nil {
		return nil, err
	}
	return toUserResponse(user), nil

}
func (s *Service) Login(ctx context.Context, email, password string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return "", ErrInvalidCredentials
	}

	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil || user == nil {
		return "", ErrInvalidCredentials
	}
	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	)
	if err != nil {
		return "", ErrInvalidCredentials
	}
	now := time.Now()

	claims := jwt.RegisteredClaims{
		Subject:   user.ID,
		Issuer:    "alphahub-api",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(2 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  claims.Subject,
		"iss":  claims.Issuer,
		"iat":  claims.IssuedAt.Unix(),
		"exp":  claims.ExpiresAt.Unix(),
		"role": user.Role,
	})
	return token.SignedString(s.jwtSecret)
}
func (s *Service) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(
		tokenString, func(token *jwt.Token) (any, error) {
			return s.jwtSecret, nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer("alphahub-api"),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}
	if _, err := claims.GetSubject(); err != nil {
		return nil, ErrInvalidToken
	}
	if sub, _ := claims.GetSubject(); sub == "" {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
func (s *Service) GetMe(ctx context.Context, tokenString string) (*UserResponse, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	userID, err := claims.GetSubject()
	if err != nil || userID == "" {
		return nil, ErrInvalidToken
	}

	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	return toUserResponse(*user), nil
}
