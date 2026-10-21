package auth

type User struct {
	ID           string `json:"id"`
	FullName     string `json:"fullName"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	CompanyID    string `json:"company_id,omitempty"`
}

type RegisterRequest struct {
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserResponse struct {
	ID        string `json:"id"`
	FullName  string `json:"fullName"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CompanyID string `json:"company_id,omitempty"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}
