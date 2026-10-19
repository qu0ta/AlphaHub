package application

type Status string

const (
	StatusDraft          Status = "DRAFT"
	StatusSubmitted      Status = "SUBMITTED"
	StatusNeedData       Status = "NEED_DATA"
	StatusInReview       Status = "IN_REVIEW"
	StatusManual         Status = "MANUAL"
	StatusOffer          Status = "OFFER"
	StatusRetentionOffer Status = "RETENTION_OFFER"
	StatusSigned         Status = "SIGNED"
	StatusDisbursed      Status = "DISBURSED"
	StatusRejected       Status = "REJECTED"
	StatusDeclined       Status = "DECLINED"
)
