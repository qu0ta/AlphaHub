package company

import "time"

type Company struct {
	ID               string    `json:"id"`
	INN              string    `json:"inn"`
	Name             string    `json:"name"`
	BusinessType     string    `json:"business_type"`
	RegistrationDate time.Time `json:"-"`
	MainOKVED        string    `json:"main_okved"`
}

type CreateRequest struct {
	INN              string `json:"inn"`
	Name             string `json:"name"`
	BusinessType     string `json:"business_type"`
	RegistrationDate string `json:"registration_date"`
	MainOKVED        string `json:"main_okved"`
}

type Response struct {
	ID               string `json:"id"`
	INN              string `json:"inn"`
	Name             string `json:"name"`
	BusinessType     string `json:"business_type"`
	RegistrationDate string `json:"registration_date"`
	MainOKVED        string `json:"main_okved"`
}

func toResponse(c *Company) Response {
	return Response{
		ID:               c.ID,
		INN:              c.INN,
		Name:             c.Name,
		BusinessType:     c.BusinessType,
		RegistrationDate: c.RegistrationDate.Format("2006-01-02"),
		MainOKVED:        c.MainOKVED,
	}
}

type InvitationResponse struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

type JoinRequest struct {
	Code string `json:"code"`
}
