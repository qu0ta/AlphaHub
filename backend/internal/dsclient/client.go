package dsclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 2 << 20

var ErrDSUnavailable = errors.New("DS service unavailable")
var ErrPDFTooLarge = errors.New("PDF exceeds 10 MiB")

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")

	u, err := url.Parse(baseURL)
	if err != nil || u == nil ||
		(u.Scheme != "http" && u.Scheme != "https") ||
		u.Host == "" {
		return nil, errors.New("invalid DS service URL")
	}

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *Client) ExtractFinancials(ctx context.Context, filename string, pdf []byte) (*ExtractFinancialsResponse, error) {
	if len(pdf) == 0 {
		return nil, errors.New("PDF is empty")
	}
	if len(pdf) > 10*1024*1024 {
		return nil, ErrPDFTooLarge
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create multipart file: %w", err)
	}

	if _, err := part.Write(pdf); err != nil {
		return nil, fmt.Errorf("write PDF: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/financials/extract", &body)

	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	data, err := c.send(req)
	if err != nil {
		return nil, err
	}

	return decodeFinancialExtraction(data)

}
func (c *Client) EvaluateScoring(ctx context.Context, input ScoringInput) (*ScoringResponse, error) {

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid scoring input: %w", err)
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode scoring input: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/v1/scoring/evaluate",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("create scoring request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	data, err := c.send(req)
	if err != nil {
		return nil, err
	}

	return decodeScoring(data)
}
func (c *Client) send(req *http.Request) (json.RawMessage, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDSUnavailable, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(
		io.LimitReader(resp.Body, maxResponseBytes+1),
	)
	if err != nil {
		return nil, fmt.Errorf("read DS response: %w", err)
	}

	if len(body) > maxResponseBytes {
		return nil, errors.New("DS response too large")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		dsErr := &DSError{
			StatusCode: resp.StatusCode,
			Code:       "INVALID_ERROR_RESPONSE",
			Message:    "DS returned an unexpected error format",
		}

		var envelope struct {
			Detail json.RawMessage `json:"detail"`
			Error  json.RawMessage `json:"error"`
		}

		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, dsErr
		}

		// Поддерживаем FastAPI detail и прежний формат error.
		for _, raw := range []json.RawMessage{
			envelope.Detail,
			envelope.Error,
		} {
			var detail struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}

			if err := json.Unmarshal(raw, &detail); err == nil &&
				detail.Code != "" &&
				detail.Message != "" {
				dsErr.Code = detail.Code
				dsErr.Message = detail.Message
				return nil, dsErr
			}
		}

		// Ошибки проверки входных полей FastAPI возвращает массивом.
		if resp.StatusCode == http.StatusUnprocessableEntity {
			var validationErrors []json.RawMessage

			if err := json.Unmarshal(
				envelope.Detail,
				&validationErrors,
			); err == nil && len(validationErrors) > 0 {
				dsErr.Code = "VALIDATION_ERROR"
				dsErr.Message = "DS rejected scoring input"
			}
		}

		return nil, dsErr
	}

	if !json.Valid(body) {
		return nil, errors.New("DS returned invalid JSON")
	}

	return json.RawMessage(body), nil
}
