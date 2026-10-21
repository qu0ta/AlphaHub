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

func (c *Client) ExtractFinancials(ctx context.Context, filename string, pdf []byte) (json.RawMessage, error) {
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
	return c.send(req)

}

func (c *Client) EvaluateScoring(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	if !json.Valid(payload) {
		return nil, errors.New("invalid scoring JSON")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/scoring/evaluate", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return c.send(req)
}

func (c *Client) send(req *http.Request) (json.RawMessage, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDSUnavailable, err)
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
		return nil, fmt.Errorf(
			"DS returned HTTP %d",
			resp.StatusCode,
		)
	}

	if !json.Valid(body) {
		return nil, errors.New("DS returned invalid JSON")
	}

	return json.RawMessage(body), nil
}
