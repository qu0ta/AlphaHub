package bank

import (
	"encoding/json"
	"errors"
	"github.com/qu0ta/AlphaHub/internal/application"
	"github.com/qu0ta/AlphaHub/internal/company"
	"github.com/qu0ta/AlphaHub/internal/documents"
	"github.com/qu0ta/AlphaHub/internal/dsclient"
	"github.com/qu0ta/AlphaHub/internal/financials"
	"time"
)

type ManualApplication struct {
	ID                     string     `json:"id"`
	CompanyName            string     `json:"company_name"`
	INN                    string     `json:"inn"`
	ProductCode            string     `json:"product_code"`
	RequestedAmountKopecks *int64     `json:"requested_amount_kopecks"`
	RequestedTermMonths    *int       `json:"requested_term_months"`
	Status                 string     `json:"status"`
	Score                  *int       `json:"score"`
	SubmittedAt            *time.Time `json:"submitted_at"`
	ManualReviewStartedAt  *time.Time `json:"manual_review_started_at"`
	AssignedEmployeeID     *string    `json:"assigned_employee_id"`
}

var (
	ErrInvalidInput = errors.New("invalid bank application input")
	ErrNotFound     = errors.New("bank application not found")
)

type ApplicationDetail struct {
	InfoRequests                      []application.InfoRequest `json:"info_requests"`
	Decisions                         []DecisionRecord          `json:"decisions"`
	Offers                            []Offer                   `json:"offers"`
	Application                       application.Application   `json:"application"`
	Company                           *company.Response         `json:"company"`
	Product                           ProductSummary            `json:"product"`
	DeclaredMonthlyDebtPaymentKopecks *int64                    `json:"declared_monthly_debt_payment_kopecks"`
	Financials                        []FinancialRecord         `json:"financials"`
	Scoring                           *ScoringDetail            `json:"scoring"`
	Documents                         []documents.Document      `json:"documents"`
	Checks                            []CheckDetail             `json:"checks"`
	History                           []StatusTransition        `json:"history"`
}

type ProductSummary struct {
	Code              string `json:"code"`
	Name              string `json:"name"`
	BaseAnnualRateBps int    `json:"base_annual_rate_bps"`
}

type FinancialRecord struct {
	financials.Month
	DataVersion       int        `json:"data_version"`
	ConfirmedAt       *time.Time `json:"confirmed_at"`
	ConfirmedByUserID *string    `json:"confirmed_by_user_id"`
}

type ScoringDetail struct {
	ID           string                      `json:"id"`
	DataVersion  int                         `json:"data_version"`
	Score        int                         `json:"score"`
	ModelVersion string                      `json:"model_version"`
	Explanation  dsclient.ScoringExplanation `json:"explanation"`
	CreatedAt    time.Time                   `json:"created_at"`
}

type CheckDetail struct {
	ID              string          `json:"id"`
	DataVersion     int             `json:"data_version"`
	CheckType       string          `json:"check_type"`
	ExecutionStatus string          `json:"execution_status"`
	Outcome         *string         `json:"outcome"`
	ReasonCode      *string         `json:"reason_code"`
	Details         json.RawMessage `json:"details"`
	StartedAt       time.Time       `json:"started_at"`
	FinishedAt      *time.Time      `json:"finished_at"`
}

type StatusTransition struct {
	ID              string    `json:"id"`
	ChangedByUserID *string   `json:"changed_by_user_id"`
	FromStatus      *string   `json:"from_status"`
	ToStatus        string    `json:"to_status"`
	Reason          string    `json:"reason"`
	CreatedAt       time.Time `json:"created_at"`
}

var (
	ErrForbidden       = errors.New("bank employee access required")
	ErrVersionConflict = errors.New("application version conflict")
	ErrNotManual       = errors.New("application is not in manual review")
	ErrAlreadyAssigned = errors.New("application is assigned to another employee")
)

type ClaimRequest struct {
	DataVersion int `json:"data_version"`
}

type Assignment struct {
	ApplicationID      string    `json:"application_id"`
	AssignedEmployeeID string    `json:"assigned_employee_id"`
	DataVersion        int       `json:"data_version"`
	Status             string    `json:"status"`
	UpdatedAt          time.Time `json:"updated_at"`
}

const (
	ActionApprove      = "APPROVE"
	ActionCounterOffer = "COUNTER_OFFER"
	ActionReject       = "REJECT"
)

var (
	ErrNotAssigned      = errors.New("claim the application before deciding")
	ErrDecisionNotReady = errors.New("current scoring and confirmed financials are required")
	ErrProductLimits    = errors.New("offer does not meet active product limits")
)

type OfferInput struct {
	AmountKopecks *int64 `json:"amount_kopecks"`
	TermMonths    *int   `json:"term_months"`
	AnnualRateBps *int   `json:"annual_rate_bps"`
}

type DecisionRequest struct {
	DataVersion int         `json:"data_version"`
	Action      string      `json:"action"`
	ReasonCode  string      `json:"reason_code"`
	Comment     string      `json:"comment"`
	ValidUntil  *time.Time  `json:"valid_until"`
	Offer       *OfferInput `json:"offer"`
}

type DecisionRecord struct {
	ID              string    `json:"id"`
	ApplicationID   string    `json:"application_id"`
	ScoringID       *string   `json:"scoring_id"`
	DecidedByUserID *string   `json:"decided_by_user_id"`
	DataVersion     int       `json:"data_version"`
	DecisionType    string    `json:"decision_type"`
	DecisionSource  string    `json:"decision_source"`
	ReasonCode      string    `json:"reason_code"`
	Comment         string    `json:"comment"`
	PolicyVersion   string    `json:"policy_version"`
	CreatedAt       time.Time `json:"created_at"`
}

type Offer struct {
	ID                    string    `json:"id"`
	DecisionID            string    `json:"decision_id"`
	Version               int       `json:"version"`
	AmountKopecks         int64     `json:"amount_kopecks"`
	TermMonths            int       `json:"term_months"`
	AnnualRateBps         int       `json:"annual_rate_bps"`
	MonthlyPaymentKopecks int64     `json:"monthly_payment_kopecks"`
	Status                string    `json:"status"`
	CreatedAt             time.Time `json:"created_at"`
	ValidUntil            time.Time `json:"valid_until"`
}

type DecisionResult struct {
	ApplicationID string         `json:"application_id"`
	DataVersion   int            `json:"data_version"`
	Status        string         `json:"status"`
	Decision      DecisionRecord `json:"decision"`
	Offer         *Offer         `json:"offer"`
}

type InfoRequestInput struct {
	DataVersion   int      `json:"data_version"`
	Description   string   `json:"description"`
	RequiredItems []string `json:"required_items"`
}

type AssignmentChangeRequest struct {
	DataVersion int             `json:"data_version"`
	EmployeeID  json.RawMessage `json:"employee_id"`
}

type ListFilter struct {
	Query    string
	Status   string
	Assigned string
	Limit    int
	Offset   int
}

type ApplicationListItem struct {
	ManualApplication
	DataVersion int `json:"data_version"`
}

type ApplicationPage struct {
	Items  []ApplicationListItem `json:"items"`
	Total  int                   `json:"total"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
}

type DecisionListItem struct {
	DecisionRecord
	CompanyName       string `json:"company_name"`
	INN               string `json:"inn"`
	ApplicationStatus string `json:"application_status"`
}

type DecisionPage struct {
	Items  []DecisionListItem `json:"items"`
	Total  int                `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

var ErrInfoRequestOpen = errors.New("an information request is already open")

type AssignmentChange struct {
	ApplicationID      string    `json:"application_id"`
	AssignedEmployeeID *string   `json:"assigned_employee_id"`
	DataVersion        int       `json:"data_version"`
	Status             string    `json:"status"`
	UpdatedAt          time.Time `json:"updated_at"`
}
