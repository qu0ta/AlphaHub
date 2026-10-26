package bank

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/qu0ta/AlphaHub/internal/application"

	"github.com/google/uuid"
)

type Service struct{ repo Repository }
type Repository interface {
	CreateInfoRequest(context.Context, string, string, InfoRequestInput) (*application.InfoRequest, error)
	ChangeAssignment(context.Context, string, string, int, *string) (*AssignmentChange, error)
	ListApplications(context.Context, string, string, ListFilter) (*ApplicationPage, error)
	ListDecisions(context.Context, string, ListFilter) (*DecisionPage, error)
	GetManual(ctx context.Context) ([]ManualApplication, error)
	GetByID(ctx context.Context, id string) (*ApplicationDetail, error)
	Claim(ctx context.Context, id, employeeID string, expectedVersion int) (*Assignment, error)
	Decide(ctx context.Context, id, employeeID string, req DecisionRequest) (*DecisionResult, error)
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }
func (s *Service) GetManual(ctx context.Context) ([]ManualApplication, error) {
	return s.repo.GetManual(ctx)
}
func (s *Service) GetByID(ctx context.Context, id string) (*ApplicationDetail, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return nil, ErrInvalidInput
	}
	return s.repo.GetByID(ctx, parsed.String())
}

func (s *Service) Claim(ctx context.Context, id, employeeID string, expectedVersion int) (*Assignment, error) {
	applicationID, err := uuid.Parse(id)
	if err != nil {
		return nil, ErrInvalidInput
	}

	employee, err := uuid.Parse(employeeID)
	if err != nil || expectedVersion < 1 {
		return nil, ErrInvalidInput
	}
	return s.repo.Claim(ctx, applicationID.String(), employee.String(), expectedVersion)
}

func (s *Service) Decide(ctx context.Context, id, employeeID string, req DecisionRequest) (*DecisionResult, error) {
	applicationID, err := uuid.Parse(id)
	if err != nil {
		return nil, ErrInvalidInput
	}

	employee, err := uuid.Parse(employeeID)
	if err != nil {
		return nil, ErrInvalidInput
	}

	req, err = validateDecisionRequest(req)
	if err != nil {
		return nil, err
	}

	return s.repo.Decide(ctx, applicationID.String(), employee.String(), req)
}

func validateDecisionRequest(req DecisionRequest) (DecisionRequest, error) {
	req.ReasonCode = strings.TrimSpace(req.ReasonCode)
	req.Comment = strings.TrimSpace(req.Comment)
	if req.DataVersion < 1 || req.ReasonCode == "" || len([]rune(req.ReasonCode)) > 100 || req.Comment == "" || len([]rune(req.Comment)) > 2000 {
		return req, ErrInvalidInput
	}
	switch req.Action {
	case ActionReject:
		if req.Offer != nil || req.ValidUntil != nil {
			return req, ErrInvalidInput
		}
	case ActionApprove, ActionCounterOffer:
		if req.ValidUntil == nil || !req.ValidUntil.After(time.Now()) {
			return req, ErrInvalidInput
		}

		if req.Action == ActionApprove && req.Offer != nil {
			return req, ErrInvalidInput
		}

		if req.Action == ActionCounterOffer {
			if req.Offer == nil || req.Offer.AmountKopecks == nil || req.Offer.TermMonths == nil || req.Offer.AnnualRateBps == nil {
				return req, ErrInvalidInput
			}

			// Ставка встречного предложения — от 15% до 30% годовых (тот же коридор, что на фронте).
			if *req.Offer.AnnualRateBps < MinOfferRateBps || *req.Offer.AnnualRateBps > MaxOfferRateBps {
				return req, ErrInvalidInput
			}

			if _, err := monthlyPayment(*req.Offer.AmountKopecks, *req.Offer.TermMonths, *req.Offer.AnnualRateBps); err != nil {
				return req, err
			}
		}
	default:
		return req, ErrInvalidInput
	}
	return req, nil
}

// Допустимый коридор ставки в решении банка, в базисных пунктах (1500 = 15,00%):
// ключевая ставка ЦБ (~14%) плюс надбавка за риск малого бизнеса.
const (
	MinOfferRateBps = 1500
	MaxOfferRateBps = 3000
)

func monthlyPayment(amount int64, months, rate int) (int64, error) {
	payment, err := application.CalculateMonthlyPayment(amount, months, rate)
	if err != nil {
		return 0, ErrInvalidInput
	}
	return payment, nil
}

func (s *Service) CreateInfoRequest(ctx context.Context, id, employee string, req InfoRequestInput) (*application.InfoRequest, error) {
	appID, err := uuid.Parse(id)
	if err != nil {
		return nil, ErrInvalidInput
	}
	actor, err := uuid.Parse(employee)
	if err != nil {
		return nil, ErrInvalidInput
	}
	req.Description = strings.TrimSpace(req.Description)
	if req.DataVersion < 1 || req.Description == "" || len([]rune(req.Description)) > 2000 || len(req.RequiredItems) < 1 || len(req.RequiredItems) > 20 {
		return nil, ErrInvalidInput
	}
	seen := map[string]bool{}
	for i, item := range req.RequiredItems {
		item = strings.TrimSpace(item)
		if item == "" || len([]rune(item)) > 500 || seen[item] {
			return nil, ErrInvalidInput
		}
		seen[item] = true
		req.RequiredItems[i] = item
	}
	return s.repo.CreateInfoRequest(ctx, appID.String(), actor.String(), req)
}
func (s *Service) ChangeAssignment(ctx context.Context, id, employee string, req AssignmentChangeRequest) (*AssignmentChange, error) {
	appID, err := uuid.Parse(id)
	if err != nil {
		return nil, ErrInvalidInput
	}
	actor, err := uuid.Parse(employee)
	if err != nil {
		return nil, ErrInvalidInput
	}
	if req.DataVersion < 1 || len(req.EmployeeID) == 0 {
		return nil, ErrInvalidInput
	}
	var target *string
	if err = json.Unmarshal(req.EmployeeID, &target); err != nil {
		return nil, ErrInvalidInput
	}
	if target != nil {
		parsed, err := uuid.Parse(*target)
		if err != nil {
			return nil, ErrInvalidInput
		}
		normalized := parsed.String()
		target = &normalized
	}
	return s.repo.ChangeAssignment(ctx, appID.String(), actor.String(), req.DataVersion, target)
}
func normalizeListFilter(f ListFilter) (ListFilter, error) {
	f.Query = strings.TrimSpace(f.Query)
	if len([]rune(f.Query)) > 200 || f.Limit < 0 || f.Limit > 100 || f.Offset < 0 || f.Offset > 1000000 {
		return f, ErrInvalidInput
	}
	if f.Limit == 0 {
		f.Limit = 20
	}
	switch f.Status {
	case "", "MANUAL", "NEEDS_INFO", "OFFERED", "REJECTED", "ACCEPTED", "OFFER_DECLINED", "OFFER_EXPIRED", "SIGNED", "DISBURSED", "CLOSED", "CANCELLED":
	default:
		return f, ErrInvalidInput
	}
	if f.Assigned != "" && f.Assigned != "mine" && f.Assigned != "unassigned" {
		id, err := uuid.Parse(f.Assigned)
		if err != nil {
			return f, ErrInvalidInput
		}
		f.Assigned = id.String()
	}
	return f, nil
}
func (s *Service) ListApplications(ctx context.Context, employee, view string, f ListFilter) (*ApplicationPage, error) {
	actor, err := uuid.Parse(employee)
	if err != nil {
		return nil, ErrInvalidInput
	}
	if view != "manual" && view != "archive" {
		return nil, ErrInvalidInput
	}
	f, err = normalizeListFilter(f)
	if err != nil {
		return nil, err
	}
	if f.Assigned == "mine" {
		f.Assigned = actor.String()
	}
	return s.repo.ListApplications(ctx, actor.String(), view, f)
}
func (s *Service) ListDecisions(ctx context.Context, employee string, f ListFilter) (*DecisionPage, error) {
	actor, err := uuid.Parse(employee)
	if err != nil {
		return nil, ErrInvalidInput
	}
	f, err = normalizeListFilter(f)
	if err != nil || f.Assigned != "" {
		return nil, ErrInvalidInput
	}
	return s.repo.ListDecisions(ctx, actor.String(), f)
}
