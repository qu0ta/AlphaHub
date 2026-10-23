package company

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrInvalidInput      = errors.New("invalid company input")
	ErrNotFound          = errors.New("company not found")
	ErrINNExists         = errors.New("company with this INN already exists")
	ErrAlreadyLinked     = errors.New("user already has a company")
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidInvitation = errors.New("invalid or expired invitation")
)

type Repository interface {
	CreateForUser(ctx context.Context, userID string, company Company) (*Company, error)
	GetByUserID(ctx context.Context, userID string) (*Company, error)
	GetByINN(ctx context.Context, inn string) (*Company, error)
	CreateInvitation(ctx context.Context, userID string, tokenHash string, expiresAt time.Time) error
	RedeemInvitation(ctx context.Context, userID string, tokenHash string) (*Company, error)
}

type Service struct {
	repo Repository
}

func NewService(repository Repository) *Service {
	return &Service{repo: repository}
}

func validINNFormat(inn string) bool {
	if len(inn) != 10 && len(inn) != 12 {
		return false
	}

	for i := 0; i < len(inn); i++ {
		if inn[i] < '0' || inn[i] > '9' {
			return false
		}
	}

	return true
}

func validText(value string, maxLength int) bool {
	return value != "" &&
		utf8.ValidString(value) &&
		utf8.RuneCountInString(value) <= maxLength
}

func hashInvitation(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
func (s *Service) Create(ctx context.Context, userID string, req CreateRequest) (*Company, error) {

	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrInvalidInput
	}
	inn := strings.TrimSpace(req.INN)
	name := strings.TrimSpace(req.Name)
	businessType := strings.TrimSpace(req.BusinessType)
	mainOKVED := strings.TrimSpace(req.MainOKVED)

	if !validINNFormat(inn) ||
		!validText(name, 255) ||
		!validText(businessType, 50) ||
		!validText(mainOKVED, 20) {
		return nil, ErrInvalidInput
	}
	stringDate := strings.TrimSpace(req.RegistrationDate)
	regDate, err := time.Parse("2006-01-02", stringDate)
	if err != nil ||
		regDate.Year() < 1 ||
		regDate.Format("2006-01-02") != stringDate {
		return nil, ErrInvalidInput
	}
	todayText := time.Now().Format("2006-01-02")
	today, err := time.Parse("2006-01-02", todayText)
	if err != nil {
		return nil, err
	}
	if regDate.After(today) {
		return nil, ErrInvalidInput
	}
	return s.repo.CreateForUser(ctx, userID, Company{
		ID:               uuid.NewString(),
		INN:              inn,
		Name:             name,
		BusinessType:     businessType,
		RegistrationDate: regDate,
		MainOKVED:        mainOKVED,
	})
}

func (s *Service) GetMe(ctx context.Context, userID string) (*Company, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrInvalidInput
	}
	return s.repo.GetByUserID(ctx, userID)
}

func (s *Service) GetByINN(ctx context.Context, inn string) (*Company, error) {
	inn = strings.TrimSpace(inn)
	if !validINNFormat(inn) {
		return nil, ErrInvalidInput
	}
	return s.repo.GetByINN(ctx, inn)
}
func (s *Service) CreateInvitation(ctx context.Context, userID string) (*InvitationResponse, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrInvalidInput
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}

	code := base64.RawURLEncoding.EncodeToString(raw)
	expiresAt := time.Now().UTC().Add(15 * time.Minute)

	err := s.repo.CreateInvitation(
		ctx,
		userID,
		hashInvitation(code),
		expiresAt,
	)
	if err != nil {
		return nil, err
	}

	return &InvitationResponse{
		Code:      code,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *Service) JoinByInvitation(ctx context.Context, userID string, code string) (*Company, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrInvalidInput
	}

	code = strings.TrimSpace(code)

	raw, err := base64.RawURLEncoding.DecodeString(code)
	if err != nil || len(raw) != 32 {
		return nil, ErrInvalidInvitation
	}

	return s.repo.RedeemInvitation(
		ctx,
		userID,
		hashInvitation(code),
	)
}
