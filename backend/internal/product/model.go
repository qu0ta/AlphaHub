package product

type Product struct {
	Code               string `json:"code"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	MinAmountKopecks   int64  `json:"min_amount_kopecks"`
	MaxAmountKopecks   int64  `json:"max_amount_kopecks"`
	MinTermMonths      int    `json:"min_term_months"`
	MaxTermMonths      int    `json:"max_term_months"`
	BaseAnnualRateBps  int    `json:"base_annual_rate_bps"`
	IsActive           bool   `json:"is_active"`
	AnnualRateBps      int    `json:"annual_rate_bps"`
	RequiresCollateral bool   `json:"requires_collateral"`
}
