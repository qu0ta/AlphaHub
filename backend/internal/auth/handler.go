package auth

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("only one JSON object is allowed")
	}

	return nil
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	data interface{},
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		log.Printf("err: %v", err)
		http.Error(w, "Invalid login request", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Email) == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}
	token, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, LoginResponse{
		AccessToken: token,
		TokenType:   "Bearer",
	})

}
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := decodeJSON(w, r, &req); err != nil {
		http.Error(w, "Invalid register request", http.StatusBadRequest)
		return
	}
	user, err := h.service.Register(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidName), errors.Is(err, ErrInvalidEmail), errors.Is(err, ErrInvalidPassword):
			http.Error(w, err.Error(), http.StatusBadRequest)

		case errors.Is(err, ErrUserExists):
			http.Error(w, "Email already registered", http.StatusConflict)

		default:
			log.Printf("register user error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusCreated, user)
}
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	parts := strings.Fields(header)

	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		http.Error(w, "Bearer token is required", http.StatusUnauthorized)
		return
	}

	user, err := h.service.GetMe(r.Context(), parts[1])
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidToken):
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)

		case errors.Is(err, ErrUserNotFound):
			http.Error(w, "User not found", http.StatusUnauthorized)

		default:
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusOK, user)
}
