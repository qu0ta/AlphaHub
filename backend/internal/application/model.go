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
