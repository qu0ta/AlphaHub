package documents

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Storage interface {
	Read(context.Context, string) ([]byte, error)
	Save(ctx context.Context, key string, data []byte) error
	Delete(ctx context.Context, key string) error
}

type LocalStorage struct {
	dir string
}

func (s *LocalStorage) Save(ctx context.Context, key string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return fmt.Errorf("create storage directory: %w", err)
	}
	file, err := os.OpenFile(
		path,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		0600,
	)
	if err != nil {
		return fmt.Errorf("create stored file: %w", err)
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return errors.Join(writeErr, closeErr)
	}
	if err := ctx.Err(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}

	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete stored file: %w", err)
	}

	return nil
}

func NewLocalStorage(dir string) *LocalStorage {
	return &LocalStorage{dir: dir}
}

func (s *LocalStorage) path(key string) (string, error) {
	if key == "" || key == "." || key == ".." ||
		filepath.Base(key) != key || strings.ContainsAny(key, `/\`) {
		return "", fmt.Errorf("invalid storage key")
	}
	return filepath.Join(s.dir, key), nil
}

func (s *LocalStorage) Read(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := s.path(key); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.Open(key)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxPDFSize {
		return nil, ErrInvalidPDF
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxPDFSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxPDFSize {
		return nil, ErrTooLarge
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return data, nil
}
