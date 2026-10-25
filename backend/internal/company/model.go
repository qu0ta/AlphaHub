package company

import "time"

type Company struct {
	ID               string    `json:"id"`
	INN              string    `json:"inn"`
	Name             string    `json:"name"`
	BusinessType     string    `json:"business_type"`
	RegistrationDate time.Time `json:"-"`
	MainOKVED        string    `json:"main_okved"`
	// Должность представителя (users.company_position) — заполняется только для своей компании
	Position *string `json:"-"`
}

type CreateRequest struct {
	INN              string `json:"inn"`
	Name             string `json:"name"`
	BusinessType     string `json:"business_type"`
	RegistrationDate string `json:"registration_date"`
	MainOKVED        string `json:"main_okved"`
	// Необязательно: должность представителя, например «Генеральный директор»
	RepresentativePosition string `json:"representative_position"`
}

// PositionRequest — изменение должности представителя (PATCH /company/me).
type PositionRequest struct {
	RepresentativePosition string `json:"representative_position"`
}

type Response struct {
	ID               string `json:"id"`
	INN              string `json:"inn"`
	Name             string `json:"name"`
	BusinessType     string `json:"business_type"`
	RegistrationDate string `json:"registration_date"`
	MainOKVED        string `json:"main_okved"`
	// Только в GET/PATCH /company/me и POST /company; в поиске по ИНН не отдаётся
	RepresentativePosition *string `json:"representative_position,omitempty"`
}

func toResponse(c *Company) Response {
	return Response{
		ID:               c.ID,
		INN:              c.INN,
		Name:             c.Name,
		BusinessType:     c.BusinessType,
		RegistrationDate: c.RegistrationDate.Format("2006-01-02"),
		MainOKVED:        c.MainOKVED,

		RepresentativePosition: c.Position,
	}
}
