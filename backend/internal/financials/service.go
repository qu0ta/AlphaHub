package financials

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	Get(ctx context.Context, applicationID, userID string) (*Financials, error)
	Update(ctx context.Context, applicationID, userID string, req UpdateRequest) (*Financials, error)
	Confirm(ctx context.Context, applicationID, userID string) (*Financials, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}
func validateIDs(applicationID, userID string) error {
	if _, err := uuid.Parse(applicationID); err != nil {
		return ErrInvalidInput
	}
	if _, err := uuid.Parse(userID); err != nil {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) Get(ctx context.Context, applicationID, userID string) (*Financials, error) {
	if err := validateIDs(applicationID, userID); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, applicationID, userID)
}

func (s *Service) Update(ctx context.Context, applicationID, userID string, req UpdateRequest) (*Financials, error) {
	if err := validateIDs(applicationID, userID); err != nil {
		return nil, err
	}
	if req.DataVersion < 1 || req.DeclaredMonthlyDebtPaymentKopecks == nil ||
		*req.DeclaredMonthlyDebtPaymentKopecks < 0 || len(req.Financials) != 6 {
		return nil, ErrInvalidInput
	}

	seen := make(map[string]bool, 6)
	for _, m := range req.Financials {
		period := strings.TrimSpace(m.PeriodMonth)

		date, err := time.Parse("2006-01", period)
		if err != nil || date.Format("2006-01") != period {
			return nil, ErrInvalidInput
		}
		if seen[period] {
			return nil, ErrInvalidInput
		}
		seen[period] = true
		if m.RevenueKopecks < 0 ||
			m.OperatingInflowKopecks < 0 ||
			m.OperatingOutflowKopecks < 0 ||
			m.TaxOutflowKopecks < 0 {
			return nil, ErrInvalidInput
		}
	}
	return s.repo.Update(ctx, applicationID, userID, req)
}
func (s *Service) Confirm(
	ctx context.Context,
	applicationID, userID string,
) (*Financials, error) {
	if err := validateIDs(applicationID, userID); err != nil {
		return nil, err
	}

	return s.repo.Confirm(ctx, applicationID, userID)
}
