package dsclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func testScoringInput() ScoringInput {
	months := make([]MonthlyFinancials, 6)

	for i := range months {
		months[i] = MonthlyFinancials{
			PeriodMonth: time.Date(
				2026, time.April, 1,
				0, 0, 0, 0, time.UTC,
			).AddDate(0, i, 0).Format("2006-01"),

			RevenueKopecks:          150000000,
			OperatingInflowKopecks:  160000000,
			OperatingOutflowKopecks: 90000000,
			TaxOutflowKopecks:       10000000,
		}
	}

	return ScoringInput{
		INN:                               "7704217370",
		AmountKopecks:                     50000000,
		TermMonths:                        12,
		AnnualRateBps:                     1990,
		ApplicationDate:                   "2026-10-23",
		Financials:                        months,
		DeclaredMonthlyDebtPaymentKopecks: 12000000,
	}
}

const testSuccessResponse = `{
	"score": 90,
	"explanation": {
		"positive_factors": ["Stable revenue"],
		"negative_factors": []
	},
	"model_version": "financial_rules_v1"
}`

// Проверяет HTTP-запрос Go-клиента без запуска настоящего DS.
func TestEvaluateScoringSuccess(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method: got %s, want POST", r.Method)
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			// Допускаем оба обсуждавшихся пути.
			// Реальный путь проверит TestLiveScoring.
			if r.URL.Path != "/scoring/evaluate" &&
				r.URL.Path != "/api/v1/scoring/evaluate" {
				t.Errorf("unexpected path: %s", r.URL.Path)
				http.NotFound(w, r)
				return
			}

			if r.Header.Get("Content-Type") != "application/json" {
				t.Error("Content-Type must be application/json")
			}

			var input ScoringInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Errorf("decode request: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if input.INN != "7704217370" ||
				input.AmountKopecks != 50000000 ||
				input.TermMonths != 12 ||
				input.AnnualRateBps != 1990 ||
				input.ApplicationDate != "2026-10-23" ||
				input.DeclaredMonthlyDebtPaymentKopecks != 12000000 {
				t.Errorf("unexpected input: %+v", input)
			}

			if len(input.Financials) != 6 {
				t.Errorf("months: got %d, want 6", len(input.Financials))
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(testSuccessResponse))
		}),
	)
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.EvaluateScoring(
		context.Background(),
		testScoringInput(),
	)
	if err != nil {
		t.Fatal(err)
	}

	assertScoringResult(t, result)
}

func assertScoringResult(t *testing.T, result *ScoringResponse) {
	t.Helper()

	if result == nil {
		t.Fatal("expected scoring result")
	}

	if result.Score != 90 {
		t.Fatalf("score: got %d, want 90", result.Score)
	}

	if result.ModelVersion == "" {
		t.Fatal("model_version is empty")
	}

	if result.Explanation.PositiveFactors == nil ||
		result.Explanation.NegativeFactors == nil {
		t.Fatal("explanation factors must be arrays, not null")
	}
}

// Невалидный запрос должен отклоняться до обращения к DS.
func TestEvaluateScoringInvalidInput(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ScoringInput)
	}{
		{
			name: "zero amount",
			mutate: func(input *ScoringInput) {
				input.AmountKopecks = 0
			},
		},
		{
			name: "negative rate",
			mutate: func(input *ScoringInput) {
				input.AnnualRateBps = -1
			},
		},
		{
			name: "missing date",
			mutate: func(input *ScoringInput) {
				input.ApplicationDate = ""
			},
		},
		{
			name: "wrong reporting period",
			mutate: func(input *ScoringInput) {
				input.Financials[0].PeriodMonth = "2025-01"
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					t.Error("invalid input reached DS")
					w.WriteHeader(http.StatusInternalServerError)
				}),
			)
			defer server.Close()

			client, err := NewClient(server.URL)
			if err != nil {
				t.Fatal(err)
			}

			input := testScoringInput()
			tc.mutate(&input)

			result, err := client.EvaluateScoring(
				context.Background(),
				input,
			)
			if err == nil || result != nil {
				t.Fatal("expected an error and no scoring result")
			}
		})
	}
}

// Проверяет сохранение HTTP-статуса и диагностического кода DS.
func TestEvaluateScoringDSErrors(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		expectCode string
	}{
		{
			name:   "calculation impossible",
			status: http.StatusUnprocessableEntity,
			body: `{
				"detail": {
					"code": "SCORING_NOT_CALCULABLE",
					"message": "Cannot calculate scoring"
				}
			}`,
			expectCode: "SCORING_NOT_CALCULABLE",
		},
		{
			name:   "validation error",
			status: http.StatusUnprocessableEntity,
			body: `{
				"detail": [{
					"type": "missing",
					"loc": ["body", "annual_rate_bps"],
					"msg": "Field required"
				}]
			}`,
			expectCode: "VALIDATION_ERROR",
		},
		{
			name:   "internal error",
			status: http.StatusInternalServerError,
			body: `{
				"detail": {
					"code": "INTERNAL_ERROR",
					"message": "Internal scoring error"
				}
			}`,
			expectCode: "INTERNAL_ERROR",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.body))
				}),
			)
			defer server.Close()

			client, err := NewClient(server.URL)
			if err != nil {
				t.Fatal(err)
			}

			result, err := client.EvaluateScoring(
				context.Background(),
				testScoringInput(),
			)
			if err == nil || result != nil {
				t.Fatal("DS error must not become a scoring result")
			}

			var dsErr *DSError
			if !errors.As(err, &dsErr) {
				t.Fatalf("expected DSError, got %v", err)
			}

			if dsErr.StatusCode != tc.status {
				t.Errorf(
					"status: got %d, want %d",
					dsErr.StatusCode,
					tc.status,
				)
			}

			if dsErr.Code != tc.expectCode {
				t.Errorf(
					"code: got %s, want %s",
					dsErr.Code,
					tc.expectCode,
				)
			}
		})
	}
}

func TestEvaluateScoringInvalidResponse(t *testing.T) {
	for _, body := range []string{
		`not JSON`,
		`{}`,
		`{"score":101,"explanation":{"positive_factors":[],"negative_factors":[]},"model_version":"test"}`,
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(body))
				}),
			)
			defer server.Close()

			client, err := NewClient(server.URL)
			if err != nil {
				t.Fatal(err)
			}

			result, err := client.EvaluateScoring(
				context.Background(),
				testScoringInput(),
			)
			if err == nil || result != nil {
				t.Fatal("expected invalid response to be rejected")
			}
		})
	}
}

// Реальное соединение Go → DS. Запускается только с DS_BASE_URL.
func TestLiveScoring(t *testing.T) {
	baseURL := os.Getenv("DS_BASE_URL")
	if baseURL == "" {
		t.Skip("set DS_BASE_URL to run the live test")
	}

	client, err := NewClient(baseURL)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		35*time.Second,
	)
	defer cancel()

	result, err := client.EvaluateScoring(ctx, testScoringInput())
	if err != nil {
		t.Fatalf("Go → DS request failed: %v", err)
	}

	assertScoringResult(t, result)
	t.Logf(
		"Go → DS OK: score=%d, model=%s",
		result.Score,
		result.ModelVersion,
	)
}
