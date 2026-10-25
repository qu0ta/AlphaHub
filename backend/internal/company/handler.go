package company

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/qu0ta/AlphaHub/internal/auth"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
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

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		http.Error(
			w,
			"Invalid company input",
			http.StatusBadRequest,
		)

	case errors.Is(err, ErrNotFound):
		http.Error(
			w,
			"Company not found",
			http.StatusNotFound,
		)

	case errors.Is(err, ErrAlreadyLinked):
		http.Error(
			w,
			"User already has a company",
			http.StatusConflict,
		)

	case errors.Is(err, ErrUserNotFound):
		http.Error(
			w,
			"User not found",
			http.StatusNotFound,
		)
	default:
		log.Printf("company error: %v", err)
		http.Error(
			w,
			"Internal server error",
			http.StatusInternalServerError,
		)
	}
}
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var req CreateRequest

	err := decodeJSON(w, r, &req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	created, err := h.service.Create(r.Context(), user.ID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	_ = json.NewEncoder(w).Encode(toResponse(created))
}
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	c, err := h.service.GetMe(r.Context(), user.ID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toResponse(c))
}
func (h *Handler) GetByINN(w http.ResponseWriter, r *http.Request) {
	inn := r.PathValue("inn")

	company, err := h.service.GetByINN(
		r.Context(),
		inn,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(toResponse(company)); err != nil {
		log.Printf("encode company response: %v", err)
	}
}

