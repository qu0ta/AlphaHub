package bank

import "time"

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
