package product

import (
	"encoding/json"
	"errors"
	"net/http"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}

func (h *Handler) GetAll(w http.ResponseWriter, r *http.Request) {
	products, err := h.service.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, products)
}

func (h *Handler) GetByCode(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	p, err := h.service.GetByCode(r.Context(), code)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidProductCode):
			http.Error(w, "Invalid product code", http.StatusBadRequest)
		case errors.Is(err, ErrProductNotFound):
			http.Error(w, "Product not found", http.StatusNotFound)
		default:
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, http.StatusOK, p)
}
