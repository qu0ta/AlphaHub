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
	"github.com/qu0ta/AlphaHub/internal/bank"
	"github.com/qu0ta/AlphaHub/internal/documents"
	"github.com/qu0ta/AlphaHub/internal/dsclient"
	"github.com/qu0ta/AlphaHub/internal/financials"
	"github.com/qu0ta/AlphaHub/internal/product"
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
	exec(`INSERT INTO companies(id,inn,name,business_type,registration_date,main_okved) VALUES ($1,'7700000098','DS Test','OOO','2020-01-01','62.01')`, companyID)
	exec(`INSERT INTO users(id,fullname,email,password_hash,company_id) VALUES ($1,'Test','ds@test.local','unused',$2)`, userID, companyID)
	exec(`INSERT INTO products (code,name,description,min_amount_kopecks,max_amount_kopecks,min_term_months,max_term_months,base_annual_rate_bps,is_active) VALUES ('test','Test','Test',1,1000000000,1,60,1990,true)`)
	extraction := dsclient.ExtractFinancialsResponse{INN: "7700000098", DeclaredMonthlyDebtPaymentKopecks: 10151981}
	now := time.Now().UTC()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for i := 6; i >= 1; i-- {
		extraction.Financials = append(extraction.Financials, dsclient.MonthlyFinancials{
			PeriodMonth: first.AddDate(0, -i, 0).Format("2006-01"), RevenueKopecks: 100000000, OperatingInflowKopecks: 110000000, OperatingOutflowKopecks: 50000000, TaxOutflowKopecks: 10000000})
	}
	var fail atomic.Bool
	var mismatch atomic.Bool
	var scoringCalls atomic.Int32
	var returnedScore atomic.Int32
	returnedScore.Store(90)
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
			_ = json.NewEncoder(w).Encode(dsclient.ScoringResponse{Score: int(returnedScore.Load()), Explanation: dsclient.ScoringExplanation{PositiveFactors: []string{"stable"}, NegativeFactors: []string{}}, ModelVersion: "test-v1"})
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
	if app.AnnualRateBps == nil || *app.AnnualRateBps != 1990 {
		t.Fatal("rate was not snapshotted on create")
	}
	// Changing the catalog must not change the rate sent for an existing application.
	exec("UPDATE products SET base_annual_rate_bps=2500 WHERE code='test'")
	t.Run("equipment catalog and product limits", func(t *testing.T) {
		catalog := product.NewPostgresRepository(pool)
		items, err := catalog.GetAll(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) < 2 {
			t.Fatal("catalog must preserve existing products")
		}
		equipment, err := catalog.GetByCode(ctx, "equipment_loan")
		if err != nil {
			t.Fatal(err)
		}
		if equipment.AnnualRateBps != 1800 || equipment.BaseAnnualRateBps != 1800 || equipment.RequiresCollateral || !equipment.IsActive || equipment.MinAmountKopecks != 50000000 || equipment.MaxAmountKopecks != 500000000 || equipment.MinTermMonths != 12 || equipment.MaxTermMonths != 36 {
			t.Fatalf("bad equipment product: %+v", equipment)
		}
		for _, c := range []struct {
			amount int64
			term   int
		}{{49999999, 12}, {500000001, 12}, {50000000, 11}, {50000000, 37}} {
			draft, err := apps.Create(ctx, userID, application.CreateRequest{ProductCode: "equipment_loan"})
			if err != nil {
				t.Fatal(err)
			}
			if draft.ProductCode != "equipment_loan" || draft.AnnualRateBps == nil || *draft.AnnualRateBps != 1800 {
				t.Fatal("wrong product snapshot")
			}
			purpose := "Equipment"
			draft, err = apps.Update(ctx, draft.ID, userID, application.UpdateRequest{CompanyID: &companyID, RequestedAmountKopecks: &c.amount, RequestedTermMonths: &c.term, Purpose: &purpose, DataVersion: &draft.DataVersion})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = apps.Submit(ctx, draft.ID, userID); !errors.Is(err, application.ErrIncompleteApplication) {
				t.Fatalf("product limits not enforced: %v", err)
			}
		}
	})
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
	if _, err = fins.Confirm(ctx, app.ID, userID, fin.FinancialDataVersion); err != nil {
		t.Fatal(err)
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
	if submitted.Status != "OFFERED" || submitted.SubmittedAt == nil {
		t.Fatal("not submitted")
	}
	exec("UPDATE products SET base_annual_rate_bps=1990 WHERE code='test'")
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

	// Exercise the actual submit transaction at every threshold boundary.
	prepareDraft := func(t *testing.T) *application.Application {
		t.Helper()
		next, err := apps.Create(ctx, userID, application.CreateRequest{ProductCode: "test"})
		if err != nil {
			t.Fatal(err)
		}
		next, err = apps.Update(ctx, next.ID, userID, application.UpdateRequest{CompanyID: &companyID, RequestedAmountKopecks: &amount, RequestedTermMonths: &term, Purpose: &purpose, DataVersion: &next.DataVersion})
		if err != nil {
			t.Fatal(err)
		}
		nextUpload := upload
		nextUpload.ApplicationID = next.ID
		if _, err = docs.Upload(ctx, nextUpload); err != nil {
			t.Fatal(err)
		}
		fin, err := fins.Get(ctx, next.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = fins.Confirm(ctx, next.ID, userID, fin.FinancialDataVersion); err != nil {
			t.Fatal(err)
		}
		next, err = apps.GetByID(ctx, next.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		return next
	}
	bankRepo := bank.NewPostgresRepository(pool)
	for _, tc := range []struct {
		name              string
		score             int32
		status            string
		decisions, offers int
	}{
		{"score 0", 0, "REJECTED", 1, 0}, {"score 39", 39, "REJECTED", 1, 0},
		{"score 40", 40, "MANUAL", 0, 0}, {"score 69", 69, "MANUAL", 0, 0},
		{"score 70", 70, "OFFERED", 1, 1}, {"score 100", 100, "OFFERED", 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			draft := prepareDraft(t)
			returnedScore.Store(tc.score)
			result, err := apps.Submit(ctx, draft.ID, userID)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != tc.status || result.DataVersion != draft.DataVersion || result.AssignedEmployeeID != nil || result.AcceptedOfferID != nil {
				t.Fatalf("wrong submission: %+v", result)
			}
			detail, err := bankRepo.GetByID(ctx, draft.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(detail.Decisions) != tc.decisions || len(detail.Offers) != tc.offers || len(detail.History) != 3 {
				t.Fatalf("wrong records: %+v", detail)
			}
			for i, transition := range detail.History {
				if i == 0 {
					if transition.FromStatus == nil || *transition.FromStatus != "DRAFT" || transition.ToStatus != "SUBMITTED" || transition.ChangedByUserID == nil {
						t.Fatal("wrong submit history")
					}
				}
				if i == 1 {
					if transition.FromStatus == nil || *transition.FromStatus != "SUBMITTED" || transition.ToStatus != "IN_REVIEW" {
						t.Fatal("wrong review history")
					}
				}
				if i == 2 {
					if transition.FromStatus == nil || *transition.FromStatus != "IN_REVIEW" || transition.ToStatus != tc.status || transition.ChangedByUserID != nil {
						t.Fatal("wrong automatic history")
					}
				}
			}
			if tc.decisions > 0 {
				decision := detail.Decisions[0]
				if decision.DecisionSource != "AUTO" || decision.DecidedByUserID != nil || decision.ScoringID == nil || detail.Scoring == nil || *decision.ScoringID != detail.Scoring.ID || decision.DataVersion != draft.DataVersion {
					t.Fatalf("wrong automatic decision: %+v", decision)
				}
				if tc.offers > 0 {
					offer := detail.Offers[0]
					payment, err := application.CalculateMonthlyPayment(amount, term, 1990)
					if err != nil {
						t.Fatal(err)
					}
					if offer.AmountKopecks != amount || offer.TermMonths != term || offer.AnnualRateBps != 1990 || offer.MonthlyPaymentKopecks != payment || offer.Status != "PENDING" || offer.ValidUntil.Sub(offer.CreatedAt) != application.AutomaticOfferValidity {
						t.Fatalf("wrong automatic offer: %+v", offer)
					}
				}
			}
			queue, err := bankRepo.GetManual(ctx)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, row := range queue {
				if row.ID == draft.ID {
					found = true
				}
			}
			if found != (tc.status == "MANUAL") {
				t.Fatal("automatic result entered manual queue")
			}
			calls := scoringCalls.Load()
			if _, err = apps.Submit(ctx, draft.ID, userID); !errors.Is(err, application.ErrNotDraft) {
				t.Fatalf("duplicate submit: %v", err)
			}
			if scoringCalls.Load() != calls {
				t.Fatal("duplicate submission called DS")
			}
		})
	}
	returnedScore.Store(90)
	t.Run("concurrent automatic submission", func(t *testing.T) {
		draft := prepareDraft(t)
		before := scoringCalls.Load()
		results := make(chan error, 2)
		start := make(chan struct{})
		for i := 0; i < 2; i++ {
			go func() { <-start; _, err := apps.Submit(ctx, draft.ID, userID); results <- err }()
		}
		close(start)
		successes, conflicts := 0, 0
		for i := 0; i < 2; i++ {
			err := <-results
			if err == nil {
				successes++
			} else if errors.Is(err, application.ErrNotDraft) {
				conflicts++
			} else {
				t.Error(err)
			}
		}
		if successes != 1 || conflicts != 1 || scoringCalls.Load() != before+1 {
			t.Fatal("duplicate scoring or submission")
		}
		detail, err := bankRepo.GetByID(ctx, draft.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(detail.Decisions) != 1 || len(detail.Offers) != 1 || len(detail.History) != 3 {
			t.Fatal("duplicate automatic records")
		}
	})
	t.Run("automatic offer failure rollback", func(t *testing.T) {
		draft := prepareDraft(t)
		exec(`CREATE FUNCTION fail_auto_offer() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test automatic offer failure'; END; $$;
   CREATE TRIGGER fail_auto_offer BEFORE INSERT ON offers FOR EACH ROW EXECUTE FUNCTION fail_auto_offer()`)
		defer func() {
			if _, err := pool.Exec(ctx, "DROP TRIGGER fail_auto_offer ON offers; DROP FUNCTION fail_auto_offer()"); err != nil {
				t.Error(err)
			}
		}()
		if _, err := apps.Submit(ctx, draft.ID, userID); err == nil {
			t.Fatal("offer error accepted")
		}
		current, err := apps.GetByID(ctx, draft.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if current.Status != "DRAFT" || current.SubmittedAt != nil {
			t.Fatal("failed offer changed application")
		}
		var scores, decisions, offers, history int
		err = pool.QueryRow(ctx, `SELECT
   (SELECT count(*) FROM scoring_results WHERE application_id=$1),
   (SELECT count(*) FROM decisions WHERE application_id=$1),
   (SELECT count(*) FROM offers o JOIN decisions d ON d.id=o.decision_id WHERE d.application_id=$1),
   (SELECT count(*) FROM status_history WHERE application_id=$1)`, draft.ID).Scan(&scores, &decisions, &offers, &history)
		if err != nil {
			t.Fatal(err)
		}
		if scores != 0 || decisions != 0 || offers != 0 || history != 0 {
			t.Fatal("partial automatic result committed")
		}
	})
	t.Run("inactive product prevents scoring", func(t *testing.T) {
		draft := prepareDraft(t)
		exec("UPDATE products SET is_active=false WHERE code='test'")
		defer func() {
			if _, err := pool.Exec(ctx, "UPDATE products SET is_active=true WHERE code='test'"); err != nil {
				t.Error(err)
			}
		}()
		before := scoringCalls.Load()
		if _, err := apps.Submit(ctx, draft.ID, userID); !errors.Is(err, application.ErrProductMissing) {
			t.Fatalf("inactive: %v", err)
		}
		if scoringCalls.Load() != before {
			t.Fatal("inactive product reached DS")
		}
	})

	// Full lifecycle fixtures share the same isolated schema and fake DS server.
	employee1, employee2 := uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO users(id,fullname,email,password_hash,role) VALUES ($1,'Bank 1','workflow-bank1@test.local','unused','BANK_EMPLOYEE')`, employee1)
	exec(`INSERT INTO users(id,fullname,email,password_hash,role) VALUES ($1,'Bank 2','workflow-bank2@test.local','unused','BANK_EMPLOYEE')`, employee2)
	bankService := bank.NewService(bankRepo)
	submitFor := func(t *testing.T, score int32) *application.Application {
		t.Helper()
		draft := prepareDraft(t)
		returnedScore.Store(score)
		result, err := apps.Submit(ctx, draft.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	pendingOffer := func(t *testing.T) (*application.Application, application.ClientOffer) {
		t.Helper()
		app := submitFor(t, 90)
		offers, err := apps.GetOffers(ctx, app.ID, userID)
		if err != nil || len(offers) != 1 {
			t.Fatalf("offers: %v %+v", err, offers)
		}
		return app, offers[0]
	}
	t.Run("client accepts offer and retries", func(t *testing.T) {
		app, offer := pendingOffer(t)
		if _, err := apps.GetOffers(ctx, app.ID, uuid.NewString()); !errors.Is(err, application.ErrNotFound) {
			t.Fatalf("foreign offers: %v", err)
		}
		if _, err := apps.RespondOffer(ctx, app.ID, uuid.NewString(), userID, application.OfferResponseRequest{DataVersion: app.DataVersion, Action: "ACCEPT"}); !errors.Is(err, application.ErrNotFound) {
			t.Fatalf("foreign offer ID: %v", err)
		}
		if _, err := apps.RespondOffer(ctx, app.ID, offer.ID, userID, application.OfferResponseRequest{DataVersion: app.DataVersion + 1, Action: "ACCEPT"}); !errors.Is(err, application.ErrVersionConflict) {
			t.Fatalf("stale response: %v", err)
		}
		req := application.OfferResponseRequest{DataVersion: app.DataVersion, Action: "ACCEPT"}
		result, err := apps.RespondOffer(ctx, app.ID, offer.ID, userID, req)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != "ACCEPTED" || result.Offer.RespondedAt == nil {
			t.Fatal("offer not accepted")
		}
		if _, err = apps.RespondOffer(ctx, app.ID, strings.ToUpper(offer.ID), userID, req); err != nil {
			t.Fatalf("safe retry: %v", err)
		}
		current, err := apps.GetByID(ctx, app.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if current.AcceptedOfferID == nil || *current.AcceptedOfferID != offer.ID {
			t.Fatal("accepted offer link missing")
		}
		detail, err := bankRepo.GetByID(ctx, app.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(detail.History) != 4 {
			t.Fatal("retry duplicated history")
		}
		page, err := bankService.ListApplications(ctx, employee1, "archive", bank.ListFilter{Query: app.ID})
		if err != nil || page.Total != 1 {
			t.Fatalf("archive: %v %+v", err, page)
		}
	})
	t.Run("client declines offer", func(t *testing.T) {
		app, offer := pendingOffer(t)
		req := application.OfferResponseRequest{DataVersion: app.DataVersion, Action: "DECLINE"}
		result, err := apps.RespondOffer(ctx, app.ID, offer.ID, userID, req)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != "OFFER_DECLINED" || result.Offer.Status != "DECLINED" {
			t.Fatal("not declined")
		}
		if _, err = apps.RespondOffer(ctx, app.ID, offer.ID, userID, req); err != nil {
			t.Fatal("decline retry", err)
		}
		req.Action = "ACCEPT"
		if _, err = apps.RespondOffer(ctx, app.ID, offer.ID, userID, req); !errors.Is(err, application.ErrWorkflowConflict) {
			t.Fatal("opposite response accepted", err)
		}
	})
	t.Run("expired offer cannot be accepted", func(t *testing.T) {
		app, offer := pendingOffer(t)
		exec(`UPDATE offers SET created_at=clock_timestamp()-interval '2 days',valid_until=clock_timestamp()-interval '1 day' WHERE id=$1`, offer.ID)
		offers, err := apps.GetOffers(ctx, app.ID, userID)
		if err != nil || offers[0].Status != "EXPIRED" {
			t.Fatalf("expiry display: %v", err)
		}
		if _, err = apps.RespondOffer(ctx, app.ID, offer.ID, userID, application.OfferResponseRequest{DataVersion: app.DataVersion, Action: "ACCEPT"}); !errors.Is(err, application.ErrOfferExpired) {
			t.Fatal("expired acceptance", err)
		}
		current, err := apps.GetByID(ctx, app.ID, userID)
		if err != nil || current.AcceptedOfferID != nil {
			t.Fatal("expired offer linked", err)
		}
		page, err := bankService.ListApplications(ctx, employee1, "archive", bank.ListFilter{Query: app.ID, Status: "OFFER_EXPIRED"})
		if err != nil || page.Total != 1 || page.Items[0].Status != "OFFER_EXPIRED" {
			t.Fatalf("expired archive: %v %+v", err, page)
		}
	})
	t.Run("competing client responses", func(t *testing.T) {
		app, offer := pendingOffer(t)
		results := make(chan error, 2)
		start := make(chan struct{})
		for _, action := range []string{"ACCEPT", "DECLINE"} {
			go func(action string) {
				<-start
				_, err := apps.RespondOffer(ctx, app.ID, offer.ID, userID, application.OfferResponseRequest{DataVersion: app.DataVersion, Action: action})
				results <- err
			}(action)
		}
		close(start)
		success, conflict := 0, 0
		for i := 0; i < 2; i++ {
			err := <-results
			if err == nil {
				success++
			} else if errors.Is(err, application.ErrWorkflowConflict) {
				conflict++
			} else {
				t.Error(err)
			}
		}
		if success != 1 || conflict != 1 {
			t.Fatal("responses were not serialized")
		}
	})
	t.Run("assignment lists and my decisions", func(t *testing.T) {
		app := submitFor(t, 65)
		if _, err := bankService.Claim(ctx, app.ID, employee1, app.DataVersion); err != nil {
			t.Fatal(err)
		}
		target, _ := json.Marshal(employee2)
		if _, err := bankService.ChangeAssignment(ctx, app.ID, employee1, bank.AssignmentChangeRequest{DataVersion: app.DataVersion, EmployeeID: target}); err != nil {
			t.Fatal(err)
		}
		if _, err := bankService.ChangeAssignment(ctx, app.ID, employee1, bank.AssignmentChangeRequest{DataVersion: app.DataVersion, EmployeeID: json.RawMessage(`null`)}); !errors.Is(err, bank.ErrAlreadyAssigned) {
			t.Fatal("former assignee kept access", err)
		}
		released, err := bankService.ChangeAssignment(ctx, app.ID, employee2, bank.AssignmentChangeRequest{DataVersion: app.DataVersion, EmployeeID: json.RawMessage(`null`)})
		if err != nil || released.AssignedEmployeeID != nil {
			t.Fatal("release failed", err)
		}
		page, err := bankService.ListApplications(ctx, employee1, "manual", bank.ListFilter{Query: app.ID, Assigned: "unassigned", Limit: 1})
		if err != nil || page.Total != 1 || len(page.Items) != 1 {
			t.Fatalf("queue: %v %+v", err, page)
		}
		page, err = bankService.ListApplications(ctx, employee1, "manual", bank.ListFilter{Query: app.ID, Limit: 1, Offset: 1})
		if err != nil || page.Total != 1 || len(page.Items) != 0 {
			t.Fatalf("offset count: %v %+v", err, page)
		}
		if _, err = bankService.Claim(ctx, app.ID, employee1, app.DataVersion); err != nil {
			t.Fatal(err)
		}
		if _, err = bankService.Decide(ctx, app.ID, employee1, bank.DecisionRequest{DataVersion: app.DataVersion, Action: bank.ActionReject, ReasonCode: "TEST", Comment: "Test rejection"}); err != nil {
			t.Fatal(err)
		}
		mine, err := bankService.ListDecisions(ctx, employee1, bank.ListFilter{Query: app.ID})
		if err != nil || mine.Total != 1 {
			t.Fatalf("my decisions: %v %+v", err, mine)
		}
		other, err := bankService.ListDecisions(ctx, employee2, bank.ListFilter{Query: app.ID})
		if err != nil || other.Total != 0 {
			t.Fatal("another employee sees my decisions", err)
		}
		injection, err := bankService.ListApplications(ctx, employee1, "archive", bank.ListFilter{Query: "'; DROP TABLE users; --"})
		if err != nil || injection.Total != 0 {
			t.Fatal("unsafe search", err)
		}
	})
	t.Run("protected PDF access", func(t *testing.T) {
		draft := prepareDraft(t)
		files, err := docs.List(ctx, draft.ID, userID)
		if err != nil || len(files) != 1 {
			t.Fatal("missing uploaded PDF", err)
		}
		if _, _, err := docs.Download(ctx, draft.ID, files[0].ID, employee1, "BANK_EMPLOYEE"); !errors.Is(err, documents.ErrDocumentNotFound) {
			t.Fatal("draft exposed to bank", err)
		}
		returnedScore.Store(65)
		if _, err = apps.Submit(ctx, draft.ID, userID); err != nil {
			t.Fatal(err)
		}
		doc, data, err := docs.Download(ctx, draft.ID, files[0].ID, employee1, "BANK_EMPLOYEE")
		if err != nil || doc.ID != files[0].ID || string(data) != "%PDF-test" {
			t.Fatal("bank PDF download", err)
		}
		if _, _, err = docs.Download(ctx, draft.ID, files[0].ID, userID, "CLIENT"); err != nil {
			t.Fatal("owner PDF", err)
		}
		if _, _, err = docs.Download(ctx, uuid.NewString(), files[0].ID, employee1, "BANK_EMPLOYEE"); !errors.Is(err, documents.ErrDocumentNotFound) {
			t.Fatal("foreign application document", err)
		}
	})
	t.Run("information request edit confirm and return", func(t *testing.T) {
		app := submitFor(t, 65)
		if _, err := bankService.Claim(ctx, app.ID, employee1, app.DataVersion); err != nil {
			t.Fatal(err)
		}
		request, err := bankService.CreateInfoRequest(ctx, app.ID, employee1, bank.InfoRequestInput{DataVersion: app.DataVersion, Description: "Updated statement", RequiredItems: []string{"Financial PDF"}})
		if err != nil {
			t.Fatal(err)
		}
		requests, err := apps.GetInfoRequests(ctx, app.ID, userID)
		if err != nil || len(requests) != 1 || requests[0].Status != "OPEN" {
			t.Fatal("client cannot read request", err)
		}
		if _, err = apps.GetInfoRequests(ctx, app.ID, uuid.NewString()); !errors.Is(err, application.ErrNotFound) {
			t.Fatal("foreign info request", err)
		}
		supporting := upload
		supporting.ApplicationID = app.ID
		supporting.DocumentType = "SUPPORTING_DOCUMENT"
		supporting.Filename = "certificate.pdf"
		if doc, err := docs.Upload(ctx, supporting); err != nil || doc.ProcessingStatus != "UPLOADED" {
			t.Fatal("supporting PDF while waiting", err)
		}
		unchanged, err := fins.Get(ctx, app.ID, userID)
		if err != nil || unchanged.DataVersion != app.DataVersion || unchanged.ConfirmedAt == nil {
			t.Fatal("supporting PDF altered financial data", err)
		}
		newUpload := upload
		newUpload.ApplicationID = app.ID
		if _, err = docs.Upload(ctx, newUpload); err != nil {
			t.Fatal("upload while waiting for info", err)
		}
		fin, err := fins.Get(ctx, app.ID, userID)
		if err != nil || fin.ConfirmedAt != nil || fin.DataVersion != app.DataVersion+1 {
			t.Fatal("new financial version", err)
		}
		response := application.InfoResponseRequest{DataVersion: app.DataVersion, ResponseComment: "New PDF uploaded"}
		if _, err = apps.RespondInfo(ctx, app.ID, request.ID, userID, response); !errors.Is(err, application.ErrVersionConflict) {
			t.Fatal("stale reply", err)
		}
		response.DataVersion = fin.DataVersion
		if _, err = apps.RespondInfo(ctx, app.ID, request.ID, userID, response); !errors.Is(err, application.ErrFinancialsUnconfirmed) {
			t.Fatal("unconfirmed reply", err)
		}
		if _, err = fins.Confirm(ctx, app.ID, userID, fin.FinancialDataVersion); err != nil {
			t.Fatal(err)
		}
		target, _ := json.Marshal(employee2)
		if _, err = bankService.ChangeAssignment(ctx, app.ID, employee1, bank.AssignmentChangeRequest{DataVersion: fin.DataVersion, EmployeeID: target}); err != nil {
			t.Fatal("transfer while waiting", err)
		}
		returnedScore.Store(90)
		fail.Store(true)
		if _, err = apps.RespondInfo(ctx, app.ID, request.ID, userID, response); err == nil {
			t.Fatal("DS failure accepted")
		}
		fail.Store(false)
		requests, err = apps.GetInfoRequests(ctx, app.ID, userID)
		if err != nil || requests[0].Status != "OPEN" {
			t.Fatal("failed rescore changed request", err)
		}
		exec("UPDATE products SET base_annual_rate_bps=2500 WHERE code='test'")
		current, err := apps.RespondInfo(ctx, app.ID, request.ID, userID, response)
		if err != nil {
			t.Fatal(err)
		}
		if current.Status != "MANUAL" || current.AssignedEmployeeID == nil || *current.AssignedEmployeeID != employee2 {
			t.Fatal("did not return to reviewer")
		}
		if _, err = apps.RespondInfo(ctx, app.ID, request.ID, userID, response); !errors.Is(err, application.ErrWorkflowConflict) {
			t.Fatal("duplicate reply", err)
		}
		detail, err := bankRepo.GetByID(ctx, app.ID)
		if err != nil || detail.Scoring == nil || detail.Scoring.Score != 90 || detail.Scoring.DataVersion != fin.DataVersion || len(detail.Offers) != 0 {
			t.Fatal("rescore did not stay manual", err)
		}
		until := time.Now().Add(time.Hour)
		if _, err = bankService.Decide(ctx, app.ID, employee2, bank.DecisionRequest{DataVersion: fin.DataVersion, Action: bank.ActionApprove, ReasonCode: "AFTER_INFO", Comment: "Reviewed updated statement", ValidUntil: &until}); err != nil {
			t.Fatal(err)
		}
		requests, err = apps.GetInfoRequests(ctx, app.ID, userID)
		if err != nil || requests[0].Status != "RESOLVED" || requests[0].ResolvedAt == nil {
			t.Fatal("request not resolved", err)
		}
	})
}
