package bank

import (
	"encoding/json"
	"log"
	"net/http"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetManual(w http.ResponseWriter, r *http.Request) {
	applications, err := h.service.GetManual(r.Context())
	if err != nil {
		log.Printf("get manual queue: %v", err)
		http.Error(
			w,
			"Internal server error",
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	if err := json.NewEncoder(w).Encode(applications); err != nil {
		log.Printf("encode manual queue: %v", err)
	}
}
