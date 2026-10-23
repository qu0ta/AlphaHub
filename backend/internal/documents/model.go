package documents

import (
	"errors"
	"time"
)

const MaxPDFSize = 10 * 1024 * 1024

var (
	ErrVersionConflict = errors.New("application changed during PDF extraction; retry upload")
	ErrINNMismatch     = errors.New("PDF INN does not match application company")
	ErrCompanyRequired = errors.New("select application company before uploading PDF")
	ErrInvalidInput    = errors.New("invalid document input")
	ErrNotFound        = errors.New("application not found")
	ErrNotDraft        = errors.New("application is not a draft")
	ErrTooLarge        = errors.New("PDF exceeds 10 MiB")
	ErrInvalidPDF      = errors.New("invalid PDF")
)

type Document struct {
	ID               string    `json:"id"`
	ApplicationID    string    `json:"application_id"`
	DocumentType     string    `json:"document_type"`
	OriginalFilename string    `json:"original_filename"`
	ProcessingStatus string    `json:"processing_status"`
	UploadedAt       time.Time `json:"uploaded_at"`
}

type UploadInput struct {
	ApplicationID string
	UserID        string
	Filename      string
	Data          []byte
}
