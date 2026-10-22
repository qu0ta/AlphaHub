package dsclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func validateFinancials(inn string, months []MonthlyFinancials, debt int64) error {
	if strings.TrimSpace(inn) == "" {
		return fmt.Errorf("inn is required")
	}

	if debt < 0 {
		return fmt.Errorf("declared_monthly_debt_payment_kopecks must be nonnegative")
	}

	if len(months) != 6 {
		return fmt.Errorf("financials must contain exactly 6 months")
	}

	seen := make(map[string]bool, 6)
	for i, month := range months {
		date, err := time.Parse("2006-01", month.PeriodMonth)
		if err != nil || date.Year() < 1 || date.Format("2006-01") != month.PeriodMonth {
			return fmt.Errorf("financials[%d].period_month must be YYYY-MM", i)
		}

		if seen[month.PeriodMonth] {
			return fmt.Errorf("duplicate financial month %s", month.PeriodMonth)
		}

		seen[month.PeriodMonth] = true
		if month.RevenueKopecks < 0 || month.OperatingInflowKopecks < 0 ||
			month.OperatingOutflowKopecks < 0 || month.TaxOutflowKopecks < 0 {
			return fmt.Errorf("financials[%d] amounts must be nonnegative", i)
		}
	}
	return nil
}

func (input ScoringInput) Validate() error {
	if input.AmountKopecks <= 0 || input.TermMonths <= 0 {
		return fmt.Errorf("amount_kopecks and term_months must be positive")
	}
	return validateFinancials(input.INN, input.Financials, input.DeclaredMonthlyDebtPaymentKopecks)
}

func requiredFields(data []byte, names ...string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage

	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}

	if object == nil {
		return nil, fmt.Errorf("expected a JSON object")
	}

	for _, name := range names {
		value, exists := object[name]

		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, fmt.Errorf("%s is required and cannot be null", name)
		}
	}
	return object, nil
}

func decodeFinancialExtraction(data []byte) (*FinancialExtractionResponse, error) {
	object, err := requiredFields(data, "inn", "financials", "declared_monthly_debt_payment_kopecks")

	if err != nil {
		return nil, fmt.Errorf("decode financials response: %w", err)
	}
	var months []json.RawMessage
	if err := json.Unmarshal(object["financials"], &months); err != nil {
		return nil, fmt.Errorf("decode financials: %w", err)
	}
	for i, month := range months {
		if _, err := requiredFields(month, "period_month", "revenue_kopecks", "operating_inflow_kopecks",
			"operating_outflow_kopecks", "tax_outflow_kopecks"); err != nil {
			return nil, fmt.Errorf("financials[%d]: %w", i, err)
		}
	}
	var result FinancialExtractionResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode financials response: %w", err)
	}
	if err := validateFinancials(result.INN, result.Financials, result.DeclaredMonthlyDebtPaymentKopecks); err != nil {
		return nil, err
	}
	return &result, nil
}

func decodeScoring(data []byte) (*ScoringResponse, error) {

	object, err := requiredFields(data, "score", "explanation", "model_version")
	if err != nil {
		return nil, fmt.Errorf("decode scoring response: %w", err)
	}

	explanation, err := requiredFields(object["explanation"], "positive_factors", "negative_factors")
	if err != nil {
		return nil, fmt.Errorf("decode explanation: %w", err)
	}

	for _, name := range []string{"positive_factors", "negative_factors"} {
		var factors []json.RawMessage
		if err := json.Unmarshal(explanation[name], &factors); err != nil {
			return nil, fmt.Errorf("%s must be an array: %w", name, err)
		}
		for _, factor := range factors {
			var text string
			if bytes.Equal(bytes.TrimSpace(factor), []byte("null")) {
				return nil, fmt.Errorf("%s items must be strings", name)
			}
			if err := json.Unmarshal(factor, &text); err != nil {
				return nil, fmt.Errorf("%s items must be strings: %w", name, err)
			}
		}
	}
	var result ScoringResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode scoring response: %w", err)
	}

	if result.Score < 0 || result.Score > 100 {
		return nil, fmt.Errorf("score must be between 0 and 100")
	}

	if strings.TrimSpace(result.ModelVersion) == "" {
		return nil, fmt.Errorf("model_version is required")
	}

	return &result, nil
}
