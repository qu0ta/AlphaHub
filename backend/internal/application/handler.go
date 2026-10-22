package application

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
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
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

func userIDFromRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok || user.ID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return "", false
	}
	return user.ID, true
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		http.Error(w, "Invalid application input", http.StatusBadRequest)

	case errors.Is(err, ErrProductMissing):
		http.Error(w, "Product not found or inactive", http.StatusBadRequest)

	case errors.Is(err, ErrNotFound):
		http.Error(w, "Application not found", http.StatusNotFound)

	case errors.Is(err, ErrNotDraft):
		http.Error(w, "Application is not a draft", http.StatusConflict)

	case errors.Is(err, ErrVersionConflict):
		http.Error(w, "Application version conflict", http.StatusConflict)

	default:
		log.Printf("application error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r)
	if !ok {
		return
	}

	var req CreateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	app, err := h.service.Create(r.Context(), userID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Location", "/api/v1/application/"+app.ID)
	writeJSON(w, http.StatusCreated, app)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r)
	if !ok {
		return
	}

	applications, err := h.service.List(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, applications)
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r)
	if !ok {
		return
	}

	app, err := h.service.GetByID(r.Context(), r.PathValue("id"), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r)
	if !ok {
		return
	}

	var req UpdateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		http.Error(w, "Invalid request boty", http.StatusBadRequest)
		return
	}

	app, err := h.service.Update(r.Context(), r.PathValue("id"), userID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}
