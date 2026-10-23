package product

import (
	"context"
	"errors"
	"strings"
)

var ErrProductNotFound = errors.New("product not found")
var ErrInvalidProductCode = errors.New("invalid product code")

type Repository interface {
	GetAll(ctx context.Context) ([]Product, error)
	GetByCode(ctx context.Context, code string) (*Product, error)
}
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) GetAll(ctx context.Context) ([]Product, error) {
	return s.repo.GetAll(ctx)
}

func (s *Service) GetByCode(ctx context.Context, code string) (*Product, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ErrInvalidProductCode
	}
	return s.repo.GetByCode(ctx, code)
}
