package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qu0ta/AlphaHub/internal/application"
	"github.com/qu0ta/AlphaHub/internal/documents"
	"github.com/qu0ta/AlphaHub/internal/dsclient"
	"github.com/qu0ta/AlphaHub/internal/financials"
)

// Uses a disposable schema; never reads or changes application rows in public.
func TestDSWorkflow(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to a local PostgreSQL instance")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "ds_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	paths, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no migrations found")
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(b)); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	companyID, userID := uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO companies(id,inn,name,business_type,registration_date,main_okved) VALUES ($1,'7700000023','DS Test','OOO','2020-01-01','62.01')`, companyID)
	exec(`INSERT INTO users(id,fullname,email,password_hash,company_id) VALUES ($1,'Test','ds@test.local','unused',$2)`, userID, companyID)
	exec(`INSERT INTO products VALUES ('test','Test','Test',1,1000000000,1,60,1990,true)`)
	extraction := dsclient.ExtractFinancialsResponse{INN: "7700000023", DeclaredMonthlyDebtPaymentKopecks: 10151981}
	now := time.Now().UTC()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for i := 6; i >= 1; i-- {
		extraction.Financials = append(extraction.Financials, dsclient.MonthlyFinancials{
			PeriodMonth: first.AddDate(0, -i, 0).Format("2006-01"), RevenueKopecks: 100000000, OperatingInflowKopecks: 110000000, OperatingOutflowKopecks: 50000000, TaxOutflowKopecks: 10000000})
	}
	var fail atomic.Bool
	var mismatch atomic.Bool
	var scoringCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/financials/extract":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			defer r.MultipartForm.RemoveAll()
			f, h, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			f.Close()
			if h.Filename != "statement.pdf" {
				t.Error("wrong upload filename")
			}
			response := extraction
			if mismatch.Load() {
				response.INN = "7700000099"
			}
			_ = json.NewEncoder(w).Encode(response)
		case "/api/v1/scoring/evaluate":
			scoringCalls.Add(1)
			var input dsclient.ScoringInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Error(err)
			}
			if input.INN != extraction.INN || input.AmountKopecks != 50000000 || input.TermMonths != 12 || input.AnnualRateBps != 1990 || input.ApplicationDate != now.Format("2006-01-02") || input.DeclaredMonthlyDebtPaymentKopecks != 10151981 {
				t.Errorf("wrong assembled input: %+v", input)
			}
			if err := input.Validate(); err != nil {
				t.Error(err)
			}
			if fail.Load() {
				w.WriteHeader(500)
				_, _ = w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"test outage"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"score":90,"explanation":{"positive_factors":["stable"],"negative_factors":[]},"model_version":"test-v1"}`))
		default:
			t.Errorf("unexpected DS path %s", r.URL.Path)
			w.WriteHeader(501)
		}
	}))
	defer server.Close()
	client, err := dsclient.NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	apps := application.NewService(application.NewPostgresRepository(pool, client))
	docs := documents.NewService(documents.NewRepository(pool), documents.NewLocalStorage(t.TempDir()), client)
	fins := financials.NewService(financials.NewRepository(pool))

	app, err := apps.Create(ctx, userID, application.CreateRequest{ProductCode: "test"})
	if err != nil {
		t.Fatal(err)
	}
	amount, term, purpose := int64(50000000), 12, "Working capital"

	app, err = apps.Update(ctx, app.ID, userID, application.UpdateRequest{CompanyID: &companyID, RequestedAmountKopecks: &amount, RequestedTermMonths: &term, Purpose: &purpose, DataVersion: &app.DataVersion})
	if err != nil {
		t.Fatal(err)
	}
	upload := documents.UploadInput{ApplicationID: app.ID, UserID: userID, Filename: "statement.pdf", Data: []byte("%PDF-test")}
	mismatch.Store(true)
	if _, err = docs.Upload(ctx, upload); !errors.Is(err, documents.ErrINNMismatch) {
		t.Fatalf("INN mismatch: %v", err)
	}
	mismatch.Store(false)
	doc, err := docs.Upload(ctx, upload)
	if err != nil {
		t.Fatal(err)
	}
	if doc.ProcessingStatus != "PROCESSED" {
		t.Fatal("document not processed")
	}
	fin, err := fins.Get(ctx, app.ID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fin.Financials) != 6 || fin.ConfirmedAt != nil || fin.DataVersion != app.DataVersion+1 || *fin.DeclaredMonthlyDebtPaymentKopecks != 10151981 {
		t.Fatalf("wrong extracted financials: %+v", fin)
	}
	if _, err = apps.Submit(ctx, app.ID, userID); !errors.Is(err, application.ErrFinancialsUnconfirmed) {
		t.Fatalf("unconfirmed submit: %v", err)
	}
	if scoringCalls.Load() != 0 {
		t.Fatal("unconfirmed financials reached DS")
	}
	if fin.FinancialDataVersion != fin.DataVersion || fin.IsConfirmationCurrent {
		t.Fatalf("wrong unconfirmed financial version: %+v", fin)
	}
	if _, err = fins.Confirm(ctx, app.ID, userID, fin.FinancialDataVersion-1); !errors.Is(err, financials.ErrVersionConflict) {
		t.Fatalf("stale confirmation accepted: %v", err)
	}
	if _, err = fins.Confirm(ctx, uuid.NewString(), userID, fin.FinancialDataVersion); !errors.Is(err, financials.ErrNotFound) {
		t.Fatalf("missing application should return not found: %v", err)
	}
	confirmed, err := fins.Confirm(ctx, app.ID, userID, fin.FinancialDataVersion)
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed.IsConfirmationCurrent || confirmed.FinancialDataVersion != fin.FinancialDataVersion {
		t.Fatalf("wrong confirmed financial version: %+v", confirmed)
	}
	fail.Store(true)
	if _, err = apps.Submit(ctx, app.ID, userID); err == nil {
		t.Fatal("DS error accepted")
	}
	current, err := apps.GetByID(ctx, app.ID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != "DRAFT" || current.SubmittedAt != nil {
		t.Fatal("failed scoring changed submission")
	}
	if _, err = apps.GetScoring(ctx, app.ID, userID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("failed score persisted: %v", err)
	}
	fail.Store(false)
	submitted, err := apps.Submit(ctx, app.ID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if submitted.Status != "IN_REVIEW" || submitted.SubmittedAt == nil {
		t.Fatal("not submitted")
	}
	score, err := apps.GetScoring(ctx, app.ID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if score.Score != 90 || score.DataVersion != fin.DataVersion || score.ModelVersion != "test-v1" {
		t.Fatalf("bad saved score %+v", score)
	}
	var snapshot dsclient.ScoringInput
	if err = json.Unmarshal(score.FeaturesSnapshot, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.ApplicationDate != submitted.SubmittedAt.UTC().Format("2006-01-02") || snapshot.AmountKopecks != amount || snapshot.AnnualRateBps != 1990 {
		t.Fatal("snapshot differs from submitted data")
	}
	if _, err = apps.GetScoring(ctx, app.ID, uuid.NewString()); !errors.Is(err, application.ErrNotFound) {
		t.Fatal("other user can read score")
	}
	calls := scoringCalls.Load()
	if _, err = apps.Submit(ctx, app.ID, userID); !errors.Is(err, application.ErrNotDraft) {
		t.Fatalf("duplicate submit: %v", err)
	}
	if scoringCalls.Load() != calls {
		t.Fatal("duplicate submit called DS")
	}
}
