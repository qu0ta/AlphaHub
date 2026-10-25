package application

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrFinancialsUnconfirmed = errors.New("confirm current financial data before submitting")
	ErrInvalidScoringPeriod  = errors.New("invalid financial reporting period or scoring parameters")
	ErrNotFound              = errors.New("application not found")
	ErrInvalidInput          = errors.New("invalid application input")
	ErrProductMissing        = errors.New("product not found or inactive")
	ErrVersionConflict       = errors.New("application version conflict")
	ErrNotDraft              = errors.New("application is not a draft")
	ErrIncompleteApplication = errors.New("application has incomplete or invalid data")
	ErrCompanyAccessDenied   = errors.New("company access denied")
)

type Repository interface {
	GetOffers(context.Context, string, string) ([]ClientOffer, error)
	RespondOffer(context.Context, string, string, string, OfferResponseRequest) (*OfferResponse, error)
	GetInfoRequests(context.Context, string, string) ([]InfoRequest, error)
	RespondInfo(context.Context, string, string, string, InfoResponseRequest) (*Application, error)
	GetScoring(context.Context, string, string) (*ScoringResult, error)
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

func (s *Service) GetScoring(ctx context.Context, id, userID string) (*ScoringResult, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidInput
	}
	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrInvalidInput
	}
	return s.repo.GetScoring(ctx, id, userID)
}

var (
	ErrWorkflowConflict = errors.New("operation is not allowed in the current state")
	ErrOfferExpired     = errors.New("offer has expired")
)

func workflowIDs(ids ...string) error {
	for _, id := range ids {
		if _, err := uuid.Parse(id); err != nil {
			return ErrInvalidInput
		}
	}
	return nil
}
func (s *Service) GetOffers(ctx context.Context, id, user string) ([]ClientOffer, error) {
	if err := workflowIDs(id, user); err != nil {
		return nil, err
	}
	return s.repo.GetOffers(ctx, id, user)
}
func (s *Service) RespondOffer(ctx context.Context, id, offerID, user string, req OfferResponseRequest) (*OfferResponse, error) {
	if err := workflowIDs(id, offerID, user); err != nil {
		return nil, err
	}
	if req.DataVersion < 1 || (req.Action != "ACCEPT" && req.Action != "DECLINE") {
		return nil, ErrInvalidInput
	}
	return s.repo.RespondOffer(ctx, uuid.MustParse(id).String(), uuid.MustParse(offerID).String(), uuid.MustParse(user).String(), req)
}
func (s *Service) GetInfoRequests(ctx context.Context, id, user string) ([]InfoRequest, error) {
	if err := workflowIDs(id, user); err != nil {
		return nil, err
	}
	return s.repo.GetInfoRequests(ctx, id, user)
}
func (s *Service) RespondInfo(ctx context.Context, id, requestID, user string, req InfoResponseRequest) (*Application, error) {
	if err := workflowIDs(id, requestID, user); err != nil {
		return nil, err
	}
	req.ResponseComment = strings.TrimSpace(req.ResponseComment)
	if req.DataVersion < 1 || req.ResponseComment == "" || len([]rune(req.ResponseComment)) > 2000 {
		return nil, ErrInvalidInput
	}
	return s.repo.RespondInfo(ctx, id, requestID, user, req)
}
