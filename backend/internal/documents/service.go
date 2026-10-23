package documents

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type Repository interface {
	Save(
		ctx context.Context,
		doc Document,
		userID string,
		storageKey string,
	) (*Document, error)

	List(
		ctx context.Context,
		applicationID string,
		userID string,
	) ([]Document, error)
}

type Service struct {
	repo    Repository
	storage Storage
}

func NewService(repo Repository, storage Storage) *Service {
	return &Service{
		repo:    repo,
		storage: storage,
	}
}

type DocumentRepository interface {
	Save(ctx context.Context, doc Document, userID string, storageKey string) (*Document, error)
	List(ctx context.Context, applicationID string, userID string) ([]Document, error)
}

func (s *Service) Upload(ctx context.Context, input UploadInput) (*Document, error) {
	if _, err := uuid.Parse(input.ApplicationID); err != nil {
		return nil, ErrInvalidInput
	}
	if _, err := uuid.Parse(input.UserID); err != nil {
		return nil, ErrInvalidInput
	}
	if len(input.Data) == 0 {
		return nil, ErrInvalidPDF
	}
	if len(input.Data) > MaxPDFSize {
		return nil, ErrTooLarge
	}

	filename := filepath.Base(strings.ReplaceAll(input.Filename, "\\", "/"))
	if filename == "" || filename == "." || len(filename) > 255 ||
		!strings.EqualFold(filepath.Ext(filename), ".pdf") {
		return nil, ErrInvalidPDF
	}
	if !bytes.HasPrefix(input.Data, []byte("%PDF-")) {
		return nil, ErrInvalidPDF
	}

	documentID := uuid.NewString()
	storageKey := documentID + ".pdf"
	doc := Document{
		ID:               documentID,
		ApplicationID:    input.ApplicationID,
		DocumentType:     "FINANCIAL_STATEMENT",
		OriginalFilename: filename,
		ProcessingStatus: "UPLOADED",
	}
	if err := s.storage.Save(ctx, storageKey, input.Data); err != nil {
		return nil, fmt.Errorf("storage PDF: %w", err)
	}

	saved, err := s.repo.Save(ctx, doc, input.UserID, storageKey)
	if err != nil {
		if cleanupErr := s.storage.Delete(context.Background(), storageKey); cleanupErr != nil {
			log.Printf("cleanup document %s: %v", documentID, cleanupErr)
		}
		return nil, err
	}
	return saved, nil

}
func (s *Service) List(ctx context.Context, applicationID string, userID string) ([]Document, error) {
	if _, err := uuid.Parse(applicationID); err != nil {
		return nil, ErrInvalidInput
	}
	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrInvalidInput
	}

	return s.repo.List(ctx, applicationID, userID)
}
