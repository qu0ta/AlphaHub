package financials

import (
	"errors"
	"time"
)

var (
	ErrInvalidInput    = errors.New("invalid financial input")
	ErrNotFound        = errors.New("financial data or application not found")
	ErrNotDraft        = errors.New("application is not a draft")
	ErrVersionConflict = errors.New("application version conflict")
)

type Month struct {
	PeriodMonth             string `json:"period_month"`
	RevenueKopecks          int64  `json:"revenue_kopecks"`
	OperatingInflowKopecks  int64  `json:"operating_inflow_kopecks"`
	OperatingOutflowKopecks int64  `json:"operating_outflow_kopecks"`
	TaxOutflowKopecks       int64  `json:"tax_outflow_kopecks"`
}

type Financials struct {
	ApplicationID                     string     `json:"application_id"`
	DataVersion                       int        `json:"data_version"`
	DeclaredMonthlyDebtPaymentKopecks *int64     `json:"declared_monthly_debt_payment_kopecks"`
	Financials                        []Month    `json:"financials"`
	ConfirmedAt                       *time.Time `json:"confirmed_at"`
	ConfirmedByUserID                 *string    `json:"confirmed_by_user_id"`
}

type UpdateRequest struct {
	DataVersion                       int     `json:"data_version"`
	DeclaredMonthlyDebtPaymentKopecks *int64  `json:"declared_monthly_debt_payment_kopecks"`
	Financials                        []Month `json:"financials"`
}
type ConfirmRequest struct {
	DataVersion int `json:"data_version"`
}
