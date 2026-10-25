package bank

import "context"

type Service struct {
	repo Repository
}
type Repository interface {
	GetManual(ctx context.Context) ([]ManualApplication, error)
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) GetManual(ctx context.Context) ([]ManualApplication, error) {
	return s.repo.GetManual(ctx)
}
