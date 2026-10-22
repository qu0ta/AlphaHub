package application

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrNotFound              = errors.New("application not found")
	ErrInvalidInput          = errors.New("invalid application input")
	ErrProductMissing        = errors.New("product not found or inactive")
	ErrVersionConflict       = errors.New("application version conflict")
	ErrNotDraft              = errors.New("application is not a draft")
	ErrIncompleteApplication = errors.New("application has incomplete or invalid data")
	ErrCompanyAccessDenied   = errors.New("company access denied")
)

type Repository interface {
	Create(ctx context.Context, userID, productCode string) (*Application, error)
	List(ctx context.Context, userID string) ([]Application, error)
	GetByID(ctx context.Context, id, userID string) (*Application, error)
	Update(ctx context.Context, id, userID string, req UpdateRequest) (*Application, error)
	Submit(ctx context.Context, id, userID string) (*Application, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}
func (s *Service) Create(ctx context.Context, userID string, req CreateRequest) (*Application, error) {
	_, err := uuid.Parse(userID)
	if err != nil {
		return nil, ErrInvalidInput
	}

	productCode := strings.TrimSpace(req.ProductCode)

	if productCode == "" || len(productCode) > 50 {
		return nil, ErrInvalidInput
	}

	return s.repo.Create(ctx, userID, productCode)
}

func (s *Service) List(ctx context.Context, userID string) ([]Application, error) {
	_, err := uuid.Parse(userID)
	if err != nil {
		return nil, ErrInvalidInput
	}

	return s.repo.List(ctx, userID)
}

func (s *Service) GetByID(ctx context.Context, id, userID string) (*Application, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidInput
	}

	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrInvalidInput
	}

	return s.repo.GetByID(ctx, id, userID)
}

func (s *Service) Update(ctx context.Context, id, userID string, req UpdateRequest) (*Application, error) {

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidInput
	}

	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrInvalidInput
	}

	if req.DataVersion == nil || *req.DataVersion < 1 {
		return nil, ErrInvalidInput
	}

	if req.CompanyID != nil {
		companyID := strings.TrimSpace(*req.CompanyID)
		if _, err := uuid.Parse(companyID); err != nil {
			return nil, ErrInvalidInput
		}
		req.CompanyID = &companyID
	}

	if req.CompanyID == nil &&
		req.RequestedAmountKopecks == nil &&
		req.RequestedTermMonths == nil &&
		req.Purpose == nil {
		return nil, ErrInvalidInput
	}

	if req.Purpose != nil {
		purpose := strings.TrimSpace(*req.Purpose)

		if len([]rune(purpose)) > 2000 {
			return nil, ErrInvalidInput
		}

		req.Purpose = &purpose
	}
	return s.repo.Update(ctx, id, userID, req)
}

func (s *Service) Submit(ctx context.Context, id, userID string) (*Application, error) {

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidInput
	}
	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrInvalidInput
	}
	return s.repo.Submit(ctx, id, userID)
}
