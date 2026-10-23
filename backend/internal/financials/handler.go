package financials

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
func userID(w http.ResponseWriter, r *http.Request) (string, bool) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return "", false
	}
	return user.ID, true
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		http.Error(w, "Invalid financial input", http.StatusBadRequest)
	case errors.Is(err, ErrNotFound):
		http.Error(w, "Financial data or application not found", http.StatusNotFound)
	case errors.Is(err, ErrNotDraft):
		http.Error(w, "Application is not a draft", http.StatusConflict)
	case errors.Is(err, ErrVersionConflict):
		http.Error(w, "Application version conflict", http.StatusConflict)
	default:
		log.Printf("financial error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func respond(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
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
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(w, r)
	if !ok {
		return
	}
	result, err := h.service.Get(r.Context(), r.PathValue("id"), uid)
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, result)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(w, r)
	if !ok {
		return
	}
	var req UpdateRequest
	err := decodeJSON(w, r, &req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result, err := h.service.Update(r.Context(), r.PathValue("id"), uid, req)

	if err != nil {
		writeError(w, err)
		return
	}

	respond(w, result)
}

func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(w, r)
	if !ok {
		return
	}

	result, err := h.service.Confirm(r.Context(), r.PathValue("id"), uid)
	if err != nil {
		writeError(w, err)
		return
	}

	respond(w, result)
}
