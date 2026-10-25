package bank

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/qu0ta/AlphaHub/internal/auth"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}
func writeServiceError(w http.ResponseWriter, err error) {

	switch {
	case errors.Is(err, ErrInvalidInput):
		http.Error(w, "Invalid bank application input", http.StatusBadRequest)
	case errors.Is(err, ErrForbidden):
		http.Error(w, "Access denied", http.StatusForbidden)
	case errors.Is(err, ErrVersionConflict):
		http.Error(w, "Application version conflict", http.StatusConflict)
	case errors.Is(err, ErrNotManual):
		http.Error(w, "Application is not in manual review", http.StatusConflict)
	case errors.Is(err, ErrAlreadyAssigned):
		http.Error(w, "Application is assigned to another employee", http.StatusConflict)
	case errors.Is(err, ErrNotAssigned):
		http.Error(w, "Claim the application before deciding", http.StatusConflict)
	case errors.Is(err, ErrDecisionNotReady):
		http.Error(w, "Current scoring and confirmed financials are required", http.StatusConflict)
	case errors.Is(err, ErrProductLimits):
		http.Error(w, "Offer does not meet active product limits", http.StatusUnprocessableEntity)
	case errors.Is(err, ErrInfoRequestOpen):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, ErrNotFound):
		http.Error(w, "Application not found", http.StatusNotFound)
	default:
		log.Printf("bank application: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("only one JSON value is allowed")
	}
	return nil
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
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user, ok := auth.UserFromContext(r.Context())
	if !ok || user.ID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if user.Role != "BANK_EMPLOYEE" {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}
	detail, err := h.service.GetByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(detail); err != nil {
		log.Printf("encode bank application: %v", err)
	}
}
func (h *Handler) Claim(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user, ok := auth.UserFromContext(r.Context())
	if !ok || user.ID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if user.Role != "BANK_EMPLOYEE" {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}
	var req ClaimRequest
	err := decodeJSON(w, r, &req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	result, err := h.service.Claim(r.Context(), r.PathValue("id"), user.ID, req.DataVersion)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("encode bank assignment: %v", err)
	}
}
func (h *Handler) Decide(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user, ok := auth.UserFromContext(r.Context())
	if !ok || user.ID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if user.Role != "BANK_EMPLOYEE" {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}
	var req DecisionRequest
	if err := decodeJSON(w, r, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	result, err := h.service.Decide(r.Context(), r.PathValue("id"), user.ID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("encode bank decision: %v", err)
	}
}

func bankUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	w.Header().Set("Cache-Control", "no-store")
	user, ok := auth.UserFromContext(r.Context())
	if !ok || user.ID == "" {
		http.Error(w, "Unauthorized", 401)
		return "", false
	}
	if user.Role != "BANK_EMPLOYEE" {
		http.Error(w, "Access denied", 403)
		return "", false
	}
	return user.ID, true
}
func bankJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("bank response: %v", err)
	}
}
func (h *Handler) CreateInfoRequest(w http.ResponseWriter, r *http.Request) {
	employee, ok := bankUser(w, r)
	if !ok {
		return
	}
	var req InfoRequestInput
	if err := decodeJSON(w, r, &req); err != nil {
		http.Error(w, "Invalid request body", 400)
		return
	}
	result, err := h.service.CreateInfoRequest(r.Context(), r.PathValue("id"), employee, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	bankJSON(w, 201, result)
}
func (h *Handler) ChangeAssignment(w http.ResponseWriter, r *http.Request) {
	employee, ok := bankUser(w, r)
	if !ok {
		return
	}
	var req AssignmentChangeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		http.Error(w, "Invalid request body", 400)
		return
	}
	result, err := h.service.ChangeAssignment(r.Context(), r.PathValue("id"), employee, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	bankJSON(w, 200, result)
}
func listFilter(r *http.Request) (ListFilter, error) {
	q := r.URL.Query()
	f := ListFilter{Query: q.Get("q"), Status: q.Get("status"), Assigned: q.Get("assigned")}
	var err error
	if value := q.Get("limit"); value != "" {
		f.Limit, err = strconv.Atoi(value)
		if err != nil || f.Limit < 1 {
			return f, ErrInvalidInput
		}
	}
	if value := q.Get("offset"); value != "" {
		f.Offset, err = strconv.Atoi(value)
		if err != nil {
			return f, ErrInvalidInput
		}
	}
	return f, nil
}
func (h *Handler) ListPage(w http.ResponseWriter, r *http.Request) { h.applicationPage(w, r, "manual") }
func (h *Handler) Archive(w http.ResponseWriter, r *http.Request)  { h.applicationPage(w, r, "archive") }
func (h *Handler) applicationPage(w http.ResponseWriter, r *http.Request, view string) {
	employee, ok := bankUser(w, r)
	if !ok {
		return
	}
	f, err := listFilter(r)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	result, err := h.service.ListApplications(r.Context(), employee, view, f)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	bankJSON(w, 200, result)
}
func (h *Handler) MyDecisions(w http.ResponseWriter, r *http.Request) {
	employee, ok := bankUser(w, r)
	if !ok {
		return
	}
	f, err := listFilter(r)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	result, err := h.service.ListDecisions(r.Context(), employee, f)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	bankJSON(w, 200, result)
}
