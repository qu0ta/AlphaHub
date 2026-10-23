package documents

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/qu0ta/AlphaHub/internal/auth"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput),
		errors.Is(err, ErrInvalidPDF):
		http.Error(w, err.Error(), http.StatusBadRequest)

	case errors.Is(err, ErrTooLarge):
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)

	case errors.Is(err, ErrNotFound):
		http.Error(w, "Application not found", http.StatusNotFound)

	case errors.Is(err, ErrNotDraft):
		http.Error(w, "Application is not a draft", http.StatusConflict)

	default:
		log.Printf("document error: %v", err)
		http.Error(
			w,
			"Internal server error",
			http.StatusInternalServerError,
		)
	}
}

func requestUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok || user.ID == "" {
		http.Error(
			w,
			"Unauthorized",
			http.StatusUnauthorized,
		)
		return "", false
	}

	return user.ID, true
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUserID(w, r)
	if !ok {
		return
	}

	contentType := strings.ToLower(
		r.Header.Get("Content-Type"),
	)
	if !strings.HasPrefix(
		contentType,
		"multipart/form-data;",
	) {
		writeError(w, ErrInvalidInput)
		return
	}

	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		MaxPDFSize+1024*1024,
	)

	if err := r.ParseMultipartForm(2 << 20); err != nil {

		if _, ok2 := errors.AsType[*http.MaxBytesError](err); ok2 {
			writeError(w, ErrTooLarge)
		} else {
			writeError(w, ErrInvalidInput)
		}
		return
	}

	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	files := r.MultipartForm.File["file"]
	if len(files) != 1 {
		writeError(w, ErrInvalidInput)
		return
	}

	header := files[0]

	if header.Size > MaxPDFSize {
		writeError(w, ErrTooLarge)
		return
	}
	if header.Size == 0 {
		writeError(w, ErrInvalidPDF)
		return
	}

	file, err := header.Open()
	if err != nil {
		writeError(w, err)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(
		io.LimitReader(file, MaxPDFSize+1),
	)
	if err != nil {
		writeError(w, err)
		return
	}
	if len(data) > MaxPDFSize {
		writeError(w, ErrTooLarge)
		return
	}

	saved, err := h.service.Upload(
		r.Context(),
		UploadInput{
			ApplicationID: r.PathValue("id"),
			UserID:        userID,
			Filename:      header.Filename,
			Data:          data,
		},
	)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(saved)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUserID(w, r)
	if !ok {
		return
	}

	documents, err := h.service.List(
		r.Context(),
		r.PathValue("id"),
		userID,
	)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(documents)
}
