package documents

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/qu0ta/AlphaHub/internal/dsclient"
)

type UploadContext struct {
	Version int
	INN     string
}

type Extractor interface {
	ExtractFinancials(context.Context, string, []byte) (*dsclient.ExtractFinancialsResponse, error)
}

type Repository interface {
	GetForDownload(context.Context, string, string, string, string) (*DownloadMetadata, error)
	PrepareUpload(context.Context, string, string) (*UploadContext, error)
	Save(
		ctx context.Context,
		doc Document,
		userID string,
		storageKey string,
		expectedVersion int,
		extraction *dsclient.ExtractFinancialsResponse,
	) (*Document, error)

	List(
		ctx context.Context,
		applicationID string,
		userID string,
	) ([]Document, error)
}

type Service struct {
	repo      Repository
	storage   Storage
	extractor Extractor
}

func NewService(repo Repository, storage Storage, extractor Extractor) *Service {
	return &Service{
		repo:      repo,
		storage:   storage,
		extractor: extractor,
	}
}

func (s *Service) Upload(ctx context.Context, input UploadInput) (*Document, error) {
	if input.DocumentType == "" {
		input.DocumentType = "FINANCIAL_STATEMENT"
	}
	if input.DocumentType != "FINANCIAL_STATEMENT" && input.DocumentType != "SUPPORTING_DOCUMENT" {
		return nil, ErrInvalidInput
	}
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

	upload, err := s.repo.PrepareUpload(ctx, input.ApplicationID, input.UserID)
	if err != nil {
		return nil, err
	}
	var extraction *dsclient.ExtractFinancialsResponse
	processingStatus := "UPLOADED"
	if input.DocumentType == "FINANCIAL_STATEMENT" {
		extraction, err = s.extractor.ExtractFinancials(ctx, filename, input.Data)
		if err != nil {
			return nil, err
		}
		if extraction.INN != upload.INN {
			return nil, ErrINNMismatch
		}
		processingStatus = "PROCESSED"
	}

	documentID := uuid.NewString()
	storageKey := documentID + ".pdf"
	doc := Document{
		ID:               documentID,
		ApplicationID:    input.ApplicationID,
		DocumentType:     input.DocumentType,
		OriginalFilename: filename,
		ProcessingStatus: processingStatus,
	}
	if err := s.storage.Save(ctx, storageKey, input.Data); err != nil {
		return nil, fmt.Errorf("storage PDF: %w", err)
	}

	saved, err := s.repo.Save(ctx, doc, input.UserID, storageKey, upload.Version, extraction)
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

func (s *Service) Download(ctx context.Context, appID, docID, user, role string) (*Document, []byte, error) {
	for _, id := range []string{appID, docID, user} {
		if _, err := uuid.Parse(id); err != nil {
			return nil, nil, ErrInvalidInput
		}
	}
	if role != "CLIENT" && role != "BANK_EMPLOYEE" {
		return nil, nil, ErrInvalidInput
	}
	metadata, err := s.repo.GetForDownload(ctx, appID, docID, user, role)
	if err != nil {
		return nil, nil, err
	}
	data, err := s.storage.Read(ctx, metadata.StorageKey)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, ErrDocumentNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	return &metadata.Document, data, nil
}
