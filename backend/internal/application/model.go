package application

import (
	"encoding/json"
	"time"
)

type Application struct {
	ID                     string     `json:"id"`
	CompanyID              *string    `json:"company_id"`
	CreatedByUserID        string     `json:"created_by_user_id"`
	ProductCode            string     `json:"product_code"`
	AnnualRateBps          *int       `json:"annual_rate_bps"`
	AssignedEmployeeID     *string    `json:"assigned_employee_id"`
	AcceptedOfferID        *string    `json:"accepted_offer_id"`
	SignedByUserID         *string    `json:"signed_by_user_id"`
	RequestedAmountKopecks *int64     `json:"requested_amount_kopecks"`
	RequestedTermMonths    *int       `json:"requested_term_months"`
	Purpose                string     `json:"purpose"`
	Status                 string     `json:"status"`
	DataVersion            int        `json:"data_version"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
	SubmittedAt            *time.Time `json:"submitted_at"`
	SignedAt               *time.Time `json:"signed_at"`
	DisbursedAt            *time.Time `json:"disbursed_at"`
}

type CreateRequest struct {
	ProductCode string `json:"product_code"`
}

type UpdateRequest struct {
	CompanyID              *string `json:"company_id,omitempty"`
	RequestedAmountKopecks *int64  `json:"requested_amount_kopecks,omitempty"`
	RequestedTermMonths    *int    `json:"requested_term_months,omitempty"`
	Purpose                *string `json:"purpose,omitempty"`
	DataVersion            *int    `json:"data_version"`
}

type ScoringResult struct {
	DataVersion      int             `json:"data_version"`
	Score            int             `json:"score"`
	ModelVersion     string          `json:"model_version"`
	Explanation      json.RawMessage `json:"explanation"`
	FeaturesSnapshot json.RawMessage `json:"features_snapshot"`
	CreatedAt        time.Time       `json:"created_at"`
}

type ClientOffer struct {
	Comment               string     `json:"comment"`
	DecisionType          string     `json:"decision_type"`
	ID                    string     `json:"id"`
	DecisionID            string     `json:"decision_id"`
	ApplicationID         string     `json:"application_id"`
	DataVersion           int        `json:"data_version"`
	Version               int        `json:"version"`
	AmountKopecks         int64      `json:"amount_kopecks"`
	TermMonths            int        `json:"term_months"`
	AnnualRateBps         int        `json:"annual_rate_bps"`
	MonthlyPaymentKopecks int64      `json:"monthly_payment_kopecks"`
	Status                string     `json:"status"`
	CreatedAt             time.Time  `json:"created_at"`
	ValidUntil            time.Time  `json:"valid_until"`
	RespondedAt           *time.Time `json:"responded_at"`
	RespondedByUserID     *string    `json:"responded_by_user_id"`
}

type OfferResponseRequest struct {
	DataVersion int    `json:"data_version"`
	Action      string `json:"action"`
}

type OfferResponse struct {
	ApplicationID string      `json:"application_id"`
	DataVersion   int         `json:"data_version"`
	Status        string      `json:"status"`
	Offer         ClientOffer `json:"offer"`
}

type InfoRequest struct {
	ID              string     `json:"id"`
	ApplicationID   string     `json:"application_id"`
	CreatedByUserID *string    `json:"created_by_user_id"`
	Description     string     `json:"description"`
	RequiredItems   []string   `json:"required_items"`
	Status          string     `json:"status"`
	ResponseComment *string    `json:"response_comment"`
	CreatedAt       time.Time  `json:"created_at"`
	RespondedAt     *time.Time `json:"responded_at"`
	ResolvedAt      *time.Time `json:"resolved_at"`
}

type InfoResponseRequest struct {
	DataVersion     int    `json:"data_version"`
	ResponseComment string `json:"response_comment"`
}
