package bank

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBankApplicationRepository(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "bank_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	files, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatal("missing migrations", err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(data)); err != nil {
			t.Fatalf("migration %s: %v", file, err)
		}
	}
	const fixtures = `
 INSERT INTO companies(id,inn,name,business_type,registration_date,main_okved)
 VALUES ('11111111-1111-4111-8111-111111111111','000000000047','Bank test','OOO','2020-01-01','62.01');
 INSERT INTO users(id,fullname,email,password_hash) VALUES ('22222222-2222-4222-8222-222222222222','Test','bank-detail@test.local','secret-hash');
 INSERT INTO products (code,name,description,min_amount_kopecks,max_amount_kopecks,min_term_months,max_term_months,base_annual_rate_bps,is_active) VALUES ('BANK_DETAIL_TEST','Bank product','Test',1,1000000000,1,60,1990,true);
 INSERT INTO applications(id,company_id,created_by_user_id,product_code,requested_amount_kopecks,requested_term_months,purpose,status,data_version,submitted_at)
 VALUES ('33333333-3333-4333-8333-333333333333','11111111-1111-4111-8111-111111111111','22222222-2222-4222-8222-222222222222','BANK_DETAIL_TEST',100000,12,'Materials','MANUAL',2,now());
 INSERT INTO financial_records(id,application_id,data_version,period_month,revenue_kopecks,operating_inflow_kopecks,operating_outflow_kopecks,tax_outflow_kopecks)
 VALUES ('44444444-4444-4444-8444-444444444444','33333333-3333-4333-8333-333333333333',2,'2026-09-01',100,100,50,10),
 ('44444444-4444-4444-8444-444444444445','33333333-3333-4333-8333-333333333333',1,'2026-08-01',999,999,50,10);
 INSERT INTO scoring_results(id,application_id,data_version,score,risk_band,model_version,features_snapshot,explanation)
 VALUES ('55555555-5555-4555-8555-555555555555','33333333-3333-4333-8333-333333333333',2,0,'UNASSESSED','test','{}','{"positive_factors":[],"negative_factors":["Test factor"]}'),
 ('55555555-5555-4555-8555-555555555556','33333333-3333-4333-8333-333333333333',1,99,'UNASSESSED','old','{}','{"positive_factors":[],"negative_factors":[]}');
 INSERT INTO documents(id,application_id,uploaded_by_user_id,document_type,original_filename,storage_key,processing_status)
 VALUES ('66666666-6666-4666-8666-666666666666','33333333-3333-4333-8333-333333333333','22222222-2222-4222-8222-222222222222','FINANCIAL_STATEMENT','test.pdf','private-storage-key','PROCESSED');
 INSERT INTO status_history(id,application_id,from_status,to_status,reason)
 VALUES ('77777777-7777-4777-8777-777777777777','33333333-3333-4333-8333-333333333333','IN_REVIEW','MANUAL','Test');
 `
	if _, err = pool.Exec(ctx, fixtures); err != nil {
		t.Fatal(err)
	}
	repo := NewPostgresRepository(pool)
	const id = "33333333-3333-4333-8333-333333333333"
	detail, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Company == nil || detail.Company.RegistrationDate != "2020-01-01" || detail.Product.Name != "Bank product" {
		t.Fatalf("missing company/product: %+v", detail)
	}
	if detail.Scoring == nil || detail.Scoring.Score != 0 || detail.Scoring.DataVersion != 2 {
		t.Fatalf("wrong scoring: %+v", detail.Scoring)
	}
	if len(detail.Financials) != 1 || detail.Financials[0].RevenueKopecks != 100 {
		t.Fatal("stale financial version included")
	}
	if len(detail.Documents) != 1 || len(detail.History) != 1 || detail.Checks == nil {
		t.Fatal("missing sections")
	}
	body, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "private-storage-key") || strings.Contains(string(body), "secret-hash") {
		t.Fatal("private data leaked")
	}
	employee1 := "88888888-8888-4888-8888-888888888881"
	const employee2 = "88888888-8888-4888-8888-888888888882"
	if _, err = pool.Exec(ctx, `INSERT INTO users(id,fullname,email,password_hash,role) VALUES
 ('88888888-8888-4888-8888-888888888881','Bank 1','employee1@test.local','unused','BANK_EMPLOYEE'),
 ('88888888-8888-4888-8888-888888888882','Bank 2','employee2@test.local','unused','BANK_EMPLOYEE')`); err != nil {
		t.Fatal(err)
	}
	t.Run("claim validation", func(t *testing.T) {
		if _, err := repo.Claim(ctx, id, "22222222-2222-4222-8222-222222222222", 2); !errors.Is(err, ErrForbidden) {
			t.Fatalf("client claim: %v", err)
		}
		if _, err := repo.Claim(ctx, id, employee1, 1); !errors.Is(err, ErrVersionConflict) {
			t.Fatalf("stale claim: %v", err)
		}
		if _, err := repo.Claim(ctx, uuid.NewString(), employee1, 2); !errors.Is(err, ErrNotFound) {
			t.Fatalf("missing claim: %v", err)
		}
	})
	t.Run("concurrent claim", func(t *testing.T) {
		type outcome struct {
			result *Assignment
			err    error
		}
		results := make(chan outcome, 2)
		start := make(chan struct{})
		for _, employee := range []string{employee1, employee2} {
			go func(employee string) {
				<-start
				result, err := repo.Claim(ctx, id, employee, 2)
				results <- outcome{result, err}
			}(employee)
		}
		close(start)
		var winner *Assignment
		conflicts := 0
		for i := 0; i < 2; i++ {
			out := <-results
			if out.err == nil {
				if winner != nil {
					t.Error("both employees claimed application")
				}
				winner = out.result
			} else if errors.Is(out.err, ErrAlreadyAssigned) {
				conflicts++
			} else {
				t.Errorf("claim error: %v", out.err)
			}
		}
		if winner == nil || conflicts != 1 {
			t.Fatal("expected one winner and one conflict")
		}
		repeated, err := repo.Claim(ctx, id, winner.AssignedEmployeeID, 2)
		if err != nil {
			t.Fatal(err)
		}
		if repeated.DataVersion != 2 || repeated.Status != "MANUAL" || !repeated.UpdatedAt.Equal(winner.UpdatedAt) {
			t.Fatal("repeat claim changed application")
		}
		detail, err := repo.GetByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if detail.Application.AssignedEmployeeID == nil || *detail.Application.AssignedEmployeeID != winner.AssignedEmployeeID {
			t.Fatal("winner not stored")
		}
	})
	// Separate submitted applications keep each decision scenario independent.
	t.Run("manual decisions", func(t *testing.T) {
		makeApp := func(t *testing.T, assigned *string) string {
			t.Helper()
			appID := uuid.NewString()
			_, err := pool.Exec(ctx, `INSERT INTO applications(id,company_id,created_by_user_id,product_code,requested_amount_kopecks,requested_term_months,purpose,status,data_version,submitted_at,assigned_employee_id,declared_monthly_debt_payment_kopecks)
    VALUES ($1,'11111111-1111-4111-8111-111111111111','22222222-2222-4222-8222-222222222222','BANK_DETAIL_TEST',100000,12,'Materials','MANUAL',2,now(),$2,1000)`, appID, assigned)
			if err != nil {
				t.Fatal(err)
			}
			_, err = pool.Exec(ctx, `INSERT INTO financial_records(id,application_id,data_version,period_month,revenue_kopecks,operating_inflow_kopecks,operating_outflow_kopecks,tax_outflow_kopecks,confirmed_at,confirmed_by_user_id)
    SELECT gen_random_uuid(),$1,2,month::date,1000000,1000000,500000,100000,now(),'22222222-2222-4222-8222-222222222222'::uuid
    FROM generate_series('2026-04-01'::date,'2026-09-01'::date,'1 month') month`, appID)
			if err != nil {
				t.Fatal(err)
			}
			_, err = pool.Exec(ctx, `INSERT INTO scoring_results(id,application_id,data_version,score,risk_band,model_version,features_snapshot,explanation)
    VALUES (gen_random_uuid(),$1,2,65,'UNASSESSED','test','{}','{"positive_factors":[],"negative_factors":[]}')`, appID)
			if err != nil {
				t.Fatal(err)
			}
			return appID
		}
		future := time.Now().Add(time.Hour)
		approve := DecisionRequest{DataVersion: 2, Action: ActionApprove, ReasonCode: "MANUAL_OK", Comment: "Reviewed", ValidUntil: &future}
		reject := DecisionRequest{DataVersion: 2, Action: ActionReject, ReasonCode: "MANUAL_REJECT", Comment: "Reason for rejection"}
		t.Run("ownership and version", func(t *testing.T) {
			appID := makeApp(t, nil)
			if _, err := repo.Decide(ctx, appID, employee1, approve); !errors.Is(err, ErrNotAssigned) {
				t.Fatalf("unclaimed: %v", err)
			}
			if _, err := repo.Claim(ctx, appID, employee1, 2); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Decide(ctx, appID, employee2, approve); !errors.Is(err, ErrAlreadyAssigned) {
				t.Fatalf("foreign employee: %v", err)
			}
			stale := approve
			stale.DataVersion = 1
			if _, err := repo.Decide(ctx, appID, employee1, stale); !errors.Is(err, ErrVersionConflict) {
				t.Fatalf("stale: %v", err)
			}
			if _, err := repo.Decide(ctx, appID, "22222222-2222-4222-8222-222222222222", approve); !errors.Is(err, ErrForbidden) {
				t.Fatalf("client: %v", err)
			}
		})
		t.Run("approve and duplicate", func(t *testing.T) {
			appID := makeApp(t, &employee1)
			result, err := repo.Decide(ctx, appID, employee1, approve)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != "OFFERED" || result.Offer == nil || result.Offer.AmountKopecks != 100000 || result.Offer.AnnualRateBps != 1990 || result.Decision.ScoringID == nil {
				t.Fatalf("wrong approval: %+v", result)
			}
			if _, err = repo.Decide(ctx, appID, employee1, approve); !errors.Is(err, ErrNotManual) {
				t.Fatalf("duplicate: %v", err)
			}
			detail, err := repo.GetByID(ctx, appID)
			if err != nil {
				t.Fatal(err)
			}
			if len(detail.Decisions) != 1 || len(detail.Offers) != 1 || len(detail.History) != 1 || detail.History[0].ToStatus != "OFFERED" {
				t.Fatal("incomplete committed decision")
			}
		})
		t.Run("counter offer", func(t *testing.T) {
			appID := makeApp(t, &employee1)
			amount, months, rate := int64(90000), 6, 1800
			req := approve
			req.Action = ActionCounterOffer
			req.Offer = &OfferInput{&amount, &months, &rate}
			result, err := repo.Decide(ctx, appID, employee1, req)
			if err != nil {
				t.Fatal(err)
			}
			expectedPayment, err := monthlyPayment(amount, months, rate)
			if err != nil {
				t.Fatal(err)
			}
			if result.Offer == nil || result.Offer.MonthlyPaymentKopecks != expectedPayment || result.Offer.AmountKopecks != amount {
				t.Fatal("wrong counter offer")
			}
			detail, err := repo.GetByID(ctx, appID)
			if err != nil {
				t.Fatal(err)
			}
			if *detail.Application.RequestedAmountKopecks != 100000 || *detail.Application.RequestedTermMonths != 12 {
				t.Fatal("offer overwrote requested terms")
			}
		})
		t.Run("unconfirmed and reject", func(t *testing.T) {
			appID := makeApp(t, &employee1)
			if _, err := pool.Exec(ctx, "UPDATE financial_records SET confirmed_at=NULL,confirmed_by_user_id=NULL WHERE application_id=$1", appID); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Decide(ctx, appID, employee1, approve); !errors.Is(err, ErrDecisionNotReady) {
				t.Fatalf("unconfirmed approval: %v", err)
			}
			result, err := repo.Decide(ctx, appID, employee1, reject)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != "REJECTED" || result.Offer != nil {
				t.Fatal("wrong rejection")
			}
			detail, err := repo.GetByID(ctx, appID)
			if err != nil {
				t.Fatal(err)
			}
			if len(detail.Decisions) != 1 || len(detail.Offers) != 0 || len(detail.History) != 1 {
				t.Fatal("wrong rejection records")
			}
		})
		t.Run("inactive product", func(t *testing.T) {
			appID := makeApp(t, &employee1)
			if _, err := pool.Exec(ctx, "UPDATE products SET is_active=false WHERE code='BANK_DETAIL_TEST'"); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := pool.Exec(ctx, "UPDATE products SET is_active=true WHERE code='BANK_DETAIL_TEST'"); err != nil {
					t.Error(err)
				}
			}()
			if _, err := repo.Decide(ctx, appID, employee1, approve); !errors.Is(err, ErrProductLimits) {
				t.Fatalf("inactive product: %v", err)
			}
		})
		t.Run("concurrent decision", func(t *testing.T) {
			appID := makeApp(t, &employee1)
			results := make(chan error, 2)
			start := make(chan struct{})
			for i := 0; i < 2; i++ {
				go func() { <-start; _, err := repo.Decide(ctx, appID, employee1, approve); results <- err }()
			}
			close(start)
			success, conflicts := 0, 0
			for i := 0; i < 2; i++ {
				err := <-results
				if err == nil {
					success++
				} else if errors.Is(err, ErrNotManual) {
					conflicts++
				} else {
					t.Errorf("decision: %v", err)
				}
			}
			if success != 1 || conflicts != 1 {
				t.Fatalf("success=%d conflicts=%d", success, conflicts)
			}
			detail, err := repo.GetByID(ctx, appID)
			if err != nil {
				t.Fatal(err)
			}
			if len(detail.Decisions) != 1 || len(detail.Offers) != 1 || len(detail.History) != 1 {
				t.Fatal("duplicate database writes")
			}
		})
		t.Run("offer failure rolls back decision", func(t *testing.T) {
			appID := makeApp(t, &employee1)
			_, err := pool.Exec(ctx, `CREATE FUNCTION fail_bank_test_offer() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test offer failure'; END; $$;
    CREATE TRIGGER fail_bank_test_offer BEFORE INSERT ON offers FOR EACH ROW EXECUTE FUNCTION fail_bank_test_offer()`)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := pool.Exec(ctx, "DROP TRIGGER fail_bank_test_offer ON offers; DROP FUNCTION fail_bank_test_offer()"); err != nil {
					t.Error(err)
				}
			}()
			if _, err := repo.Decide(ctx, appID, employee1, approve); err == nil {
				t.Fatal("offer failure accepted")
			}
			detail, err := repo.GetByID(ctx, appID)
			if err != nil {
				t.Fatal(err)
			}
			if detail.Application.Status != "MANUAL" || len(detail.Decisions) != 0 || len(detail.Offers) != 0 || len(detail.History) != 0 {
				t.Fatal("partial transaction committed")
			}
		})
	})
	if _, err = pool.Exec(ctx, "UPDATE applications SET status='IN_REVIEW' WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Claim(ctx, id, employee1, 2); !errors.Is(err, ErrNotManual) {
		t.Fatalf("non-manual claim: %v", err)
	}
	if _, err = repo.GetByID(ctx, uuid.NewString()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if _, err = pool.Exec(ctx, "UPDATE applications SET status='DRAFT',submitted_at=NULL WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.GetByID(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("draft visible: %v", err)
	}
	if _, err = repo.Claim(ctx, id, employee1, 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("draft claim: %v", err)
	}
}
