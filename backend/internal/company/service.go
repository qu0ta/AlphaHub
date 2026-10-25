package company

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrInvalidInput  = errors.New("invalid company input")
	ErrNotFound      = errors.New("company not found")
	ErrAlreadyLinked = errors.New("user already has a company")
	ErrUserNotFound  = errors.New("user not found")
)

type Repository interface {
	// CreateForUser привязывает пользователя к компании по ИНН:
	// если компании с таким ИНН ещё нет — создаёт её, если есть — присоединяет к существующей.
	CreateForUser(ctx context.Context, userID string, company Company) (*Company, error)
	GetByUserID(ctx context.Context, userID string) (*Company, error)
	GetByINN(ctx context.Context, inn string) (*Company, error)
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
