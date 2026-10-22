package dsclient

type MonthlyFinancials struct {
	PeriodMonth             string `json:"period_month"`
	RevenueKopecks          int64  `json:"revenue_kopecks"`
	OperatingInflowKopecks  int64  `json:"operating_inflow_kopecks"`
	OperatingOutflowKopecks int64  `json:"operating_outflow_kopecks"`
	TaxOutflowKopecks       int64  `json:"tax_outflow_kopecks"`
}

type FinancialMonth = MonthlyFinancials

type FinancialExtractionResponse struct {
	INN                               string              `json:"inn"`
	Financials                        []MonthlyFinancials `json:"financials"`
	DeclaredMonthlyDebtPaymentKopecks int64               `json:"declared_monthly_debt_payment_kopecks"`
}

type ExtractFinancialsResponse = FinancialExtractionResponse

type ScoringInput struct {
	INN                               string              `json:"inn"`
	AmountKopecks                     int64               `json:"amount_kopecks"`
	TermMonths                        int                 `json:"term_months"`
	Financials                        []MonthlyFinancials `json:"financials"`
	DeclaredMonthlyDebtPaymentKopecks int64               `json:"declared_monthly_debt_payment_kopecks"`
	AnnualRateBps                     int                 `json:"annual_rate_bps"`
	ApplicationDate                   string              `json:"application_date"`
}

type ScoringExplanation struct {
	PositiveFactors []string `json:"positive_factors"`
	NegativeFactors []string `json:"negative_factors"`
}

type ScoringResponse struct {
	Score        int                `json:"score"`
	Explanation  ScoringExplanation `json:"explanation"`
	ModelVersion string             `json:"model_version"`
}
