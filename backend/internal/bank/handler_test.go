package bank

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/qu0ta/AlphaHub/internal/application"
	"github.com/qu0ta/AlphaHub/internal/auth"
	"github.com/qu0ta/AlphaHub/internal/documents"
	"github.com/qu0ta/AlphaHub/internal/dsclient"
)

type detailRepoStub struct {
	Repository
	result *ApplicationDetail
	err    error
	calls  int
}

func (r *detailRepoStub) GetByID(_ context.Context, _ string) (*ApplicationDetail, error) {
	r.calls++
	return r.result, r.err
}

type unusedAuthRepo struct{ auth.UserRepository }

func TestGetByIDAccessAndErrors(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	key := strings.Repeat("test-secret-", 4)
	authService, err := auth.NewService(&unusedAuthRepo{}, key)
	if err != nil {
		t.Fatal(err)
	}
	authHandler := auth.NewHandler(authService)
	cases := []struct {
		name, role, path string
		repoErr          error
		want, calls      int
	}{
		{name: "anonymous", path: id, want: 401},
		{name: "client", role: "CLIENT", path: id, want: 403},
		{name: "invalid UUID", role: "BANK_EMPLOYEE", path: "invalid", want: 400},
		{name: "missing or draft", role: "BANK_EMPLOYEE", path: id, repoErr: ErrNotFound, want: 404, calls: 1},
		{name: "database failure", role: "BANK_EMPLOYEE", path: id, repoErr: errors.New("private database detail"), want: 500, calls: 1},
		{name: "employee", role: "BANK_EMPLOYEE", path: id, want: 200, calls: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &detailRepoStub{result: &ApplicationDetail{}, err: tc.repoErr}
			handler := NewHandler(NewService(repo))
			mux := http.NewServeMux()
			mux.Handle("GET /api/v1/bank/applications/{id}", authHandler.RequireAuth(auth.RequireRole("BANK_EMPLOYEE")(http.HandlerFunc(handler.GetByID))))
			req := httptest.NewRequest("GET", "/api/v1/bank/applications/"+tc.path, nil)
			if tc.role != "" {
				token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"sub": id, "role": tc.role, "iss": "alphahub-api", "exp": time.Now().Add(time.Hour).Unix(),
				}).SignedString([]byte(key))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Authorization", "Bearer "+token)
			}
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, req)
			if recorder.Code != tc.want || repo.calls != tc.calls {
				t.Fatalf("status=%d calls=%d body=%s", recorder.Code, repo.calls, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), "private database detail") {
				t.Fatal("internal error leaked")
			}
			if tc.want == 200 {
				if recorder.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("missing no-store")
				}
				var detail ApplicationDetail
				if err := json.Unmarshal(recorder.Body.Bytes(), &detail); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestGetByIDRequiresBankContext(t *testing.T) {
	repo := &detailRepoStub{}
	recorder := httptest.NewRecorder()
	NewHandler(NewService(repo)).GetByID(recorder, httptest.NewRequest("GET", "/", nil))
	if recorder.Code != 401 || repo.calls != 0 {
		t.Fatal("unguarded handler access")
	}
}

type claimRepoStub struct {
	Repository
	calls        int
	id, employee string
	version      int
	err          error
}

func (r *claimRepoStub) Claim(_ context.Context, id, employee string, version int) (*Assignment, error) {
	r.calls++
	r.id = id
	r.employee = employee
	r.version = version
	return &Assignment{ApplicationID: id, AssignedEmployeeID: employee, DataVersion: version, Status: "MANUAL"}, r.err
}

func TestClaimAccessValidationAndConflicts(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	const employee = "22222222-2222-4222-8222-222222222222"
	key := strings.Repeat("test-secret-", 4)
	service, err := auth.NewService(&unusedAuthRepo{}, key)
	if err != nil {
		t.Fatal(err)
	}
	authHandler := auth.NewHandler(service)
	cases := []struct {
		name, role, path, body string
		repoErr                error
		status, calls          int
	}{
		{name: "no token", path: id, body: `{"data_version":2}`, status: 401},
		{name: "client", role: "CLIENT", path: id, body: `{"data_version":2}`, status: 403},
		{name: "empty body", role: "BANK_EMPLOYEE", path: id, status: 400},
		{name: "null body", role: "BANK_EMPLOYEE", path: id, body: `null`, status: 400},
		{name: "missing version", role: "BANK_EMPLOYEE", path: id, body: `{}`, status: 400},
		{name: "invalid version", role: "BANK_EMPLOYEE", path: id, body: `{"data_version":-1}`, status: 400},
		{name: "invalid UUID", role: "BANK_EMPLOYEE", path: "bad", body: `{"data_version":2}`, status: 400},
		{name: "unknown employee field", role: "BANK_EMPLOYEE", path: id, body: `{"data_version":2,"employee_id":"other"}`, status: 400},
		{name: "two objects", role: "BANK_EMPLOYEE", path: id, body: `{"data_version":2}{}`, status: 400},
		{name: "oversize", role: "BANK_EMPLOYEE", path: id, body: `{"data_version":2}` + strings.Repeat(" ", 16*1024), status: 400},
		{name: "stale", role: "BANK_EMPLOYEE", path: id, body: `{"data_version":2}`, repoErr: ErrVersionConflict, status: 409, calls: 1},
		{name: "other employee", role: "BANK_EMPLOYEE", path: id, body: `{"data_version":2}`, repoErr: ErrAlreadyAssigned, status: 409, calls: 1},
		{name: "not manual", role: "BANK_EMPLOYEE", path: id, body: `{"data_version":2}`, repoErr: ErrNotManual, status: 409, calls: 1},
		{name: "missing", role: "BANK_EMPLOYEE", path: id, body: `{"data_version":2}`, repoErr: ErrNotFound, status: 404, calls: 1},
		{name: "revoked role", role: "BANK_EMPLOYEE", path: id, body: `{"data_version":2}`, repoErr: ErrForbidden, status: 403, calls: 1},
		{name: "success", role: "BANK_EMPLOYEE", path: id, body: `{"data_version":2}`, status: 200, calls: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &claimRepoStub{err: tc.repoErr}
			handler := NewHandler(NewService(repo))
			mux := http.NewServeMux()
			mux.Handle("POST /api/v1/bank/applications/{id}/claim", authHandler.RequireAuth(auth.RequireRole("BANK_EMPLOYEE")(http.HandlerFunc(handler.Claim))))
			req := httptest.NewRequest("POST", "/api/v1/bank/applications/"+tc.path+"/claim", strings.NewReader(tc.body))
			if tc.role != "" {
				token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": employee, "role": tc.role, "iss": "alphahub-api", "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte(key))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Authorization", "Bearer "+token)
			}
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, req)
			if recorder.Code != tc.status || repo.calls != tc.calls {
				t.Fatalf("status=%d calls=%d body=%s", recorder.Code, repo.calls, recorder.Body.String())
			}
			if repo.calls > 0 && (repo.employee != employee || repo.id != id || repo.version != 2) {
				t.Fatalf("wrong repository arguments: %+v", repo)
			}
			if tc.status == 200 {
				var result Assignment
				if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result.AssignedEmployeeID != employee || result.Status != "MANUAL" || recorder.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("invalid assignment response")
				}
			}
		})
	}
}

func TestClaimRequiresBankContext(t *testing.T) {
	repo := &claimRepoStub{}
	recorder := httptest.NewRecorder()
	NewHandler(NewService(repo)).Claim(recorder, httptest.NewRequest("POST", "/", strings.NewReader(`{"data_version":2}`)))
	if recorder.Code != 401 || repo.calls != 0 {
		t.Fatal("unguarded claim access")
	}
}

func TestMonthlyPayment(t *testing.T) {
	cases := []struct {
		amount       int64
		months, rate int
		want         int64
		invalid      bool
	}{
		{100, 3, 0, 34, false}, {100000, 1, 1200, 101000, false}, {10000000, 12, 1200, 888488, false},
		{9223372036854775807, 1, 0, 9223372036854775807, false}, {9223372036854775807, 1, 1200, 0, true},
		{0, 12, 1000, 0, true}, {100, 0, 0, 0, true}, {100, 601, 0, 0, true}, {100, 12, -1, 0, true},
	}
	for _, tc := range cases {
		got, err := monthlyPayment(tc.amount, tc.months, tc.rate)
		if tc.invalid {
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected invalid: %+v got=%d err=%v", tc, got, err)
			}
		} else if err != nil || got != tc.want {
			t.Fatalf("payment %+v got=%d err=%v", tc, got, err)
		}
	}
}

type decisionRepoStub struct {
	Repository
	calls    int
	employee string
	err      error
}

func (r *decisionRepoStub) Decide(_ context.Context, id, employee string, req DecisionRequest) (*DecisionResult, error) {
	r.calls++
	r.employee = employee
	return &DecisionResult{ApplicationID: id, DataVersion: req.DataVersion, Status: "REJECTED"}, r.err
}

func TestDecideAccessAndValidation(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	key := strings.Repeat("test-secret-", 4)
	service, err := auth.NewService(&unusedAuthRepo{}, key)
	if err != nil {
		t.Fatal(err)
	}
	authHandler := auth.NewHandler(service)
	valid := `{"data_version":2,"action":"REJECT","reason_code":"MANUAL_REJECT","comment":"Test reason"}`
	cases := []struct {
		name, role, body string
		err              error
		status, calls    int
	}{
		{name: "anonymous", body: valid, status: 401},
		{name: "client", role: "CLIENT", body: valid, status: 403},
		{name: "null", role: "BANK_EMPLOYEE", body: `null`, status: 400},
		{name: "unknown action", role: "BANK_EMPLOYEE", body: `{"data_version":2,"action":"UNKNOWN","reason_code":"X","comment":"Reason"}`, status: 400},
		{name: "missing comment", role: "BANK_EMPLOYEE", body: `{"data_version":2,"action":"REJECT","reason_code":"X"}`, status: 400},
		{name: "unknown field", role: "BANK_EMPLOYEE", body: strings.TrimSuffix(valid, "}") + `,"employee_id":"other"}`, status: 400},
		{name: "trailing value", role: "BANK_EMPLOYEE", body: valid + `{}`, status: 400},
		{name: "oversized", role: "BANK_EMPLOYEE", body: valid + strings.Repeat(" ", 16*1024), status: 400},
		{name: "unclaimed", role: "BANK_EMPLOYEE", body: valid, err: ErrNotAssigned, status: 409, calls: 1},
		{name: "other employee", role: "BANK_EMPLOYEE", body: valid, err: ErrAlreadyAssigned, status: 409, calls: 1},
		{name: "stale", role: "BANK_EMPLOYEE", body: valid, err: ErrVersionConflict, status: 409, calls: 1},
		{name: "missing data", role: "BANK_EMPLOYEE", body: valid, err: ErrDecisionNotReady, status: 409, calls: 1},
		{name: "product limits", role: "BANK_EMPLOYEE", body: valid, err: ErrProductLimits, status: 422, calls: 1},
		{name: "success", role: "BANK_EMPLOYEE", body: valid, status: 201, calls: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &decisionRepoStub{err: tc.err}
			handler := NewHandler(NewService(repo))
			mux := http.NewServeMux()
			mux.Handle("POST /api/v1/bank/applications/{id}/decisions", authHandler.RequireAuth(auth.RequireRole("BANK_EMPLOYEE")(http.HandlerFunc(handler.Decide))))
			req := httptest.NewRequest("POST", "/api/v1/bank/applications/"+id+"/decisions", strings.NewReader(tc.body))
			if tc.role != "" {
				token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": id, "role": tc.role, "iss": "alphahub-api", "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte(key))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Authorization", "Bearer "+token)
			}
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, req)
			if recorder.Code != tc.status || repo.calls != tc.calls {
				t.Fatalf("status=%d calls=%d body=%s", recorder.Code, repo.calls, recorder.Body.String())
			}
			if repo.calls > 0 && repo.employee != id {
				t.Fatal("actor differs from JWT")
			}
		})
	}
}

func TestDecisionTermsValidation(t *testing.T) {
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	amount, months, rate := int64(100000), 12, 0
	cases := []struct {
		name  string
		req   DecisionRequest
		valid bool
	}{
		{"approval", DecisionRequest{DataVersion: 2, Action: ActionApprove, ReasonCode: "OK", Comment: "Approved", ValidUntil: &future}, true},
		{"counter zero rate", DecisionRequest{DataVersion: 2, Action: ActionCounterOffer, ReasonCode: "ALT", Comment: "Alternative", ValidUntil: &future, Offer: &OfferInput{&amount, &months, &rate}}, true},
		{"approve overrides", DecisionRequest{DataVersion: 2, Action: ActionApprove, ReasonCode: "OK", Comment: "Approved", ValidUntil: &future, Offer: &OfferInput{&amount, &months, &rate}}, false},
		{"expired", DecisionRequest{DataVersion: 2, Action: ActionApprove, ReasonCode: "OK", Comment: "Approved", ValidUntil: &past}, false},
		{"counter missing fields", DecisionRequest{DataVersion: 2, Action: ActionCounterOffer, ReasonCode: "ALT", Comment: "Alternative", ValidUntil: &future, Offer: &OfferInput{}}, false},
		{"reject offer", DecisionRequest{DataVersion: 2, Action: ActionReject, ReasonCode: "NO", Comment: "Rejected", Offer: &OfferInput{}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateDecisionRequest(tc.req)
			if (err == nil) != tc.valid {
				t.Fatalf("validation err=%v", err)
			}
		})
	}
}

type workflowRepoStub struct {
	Repository
	calls  int
	filter ListFilter
	target *string
}

func (r *workflowRepoStub) CreateInfoRequest(_ context.Context, id, actor string, req InfoRequestInput) (*application.InfoRequest, error) {
	r.calls++
	return &application.InfoRequest{ApplicationID: id, Description: req.Description, RequiredItems: req.RequiredItems, Status: "OPEN"}, nil
}
func (r *workflowRepoStub) ChangeAssignment(_ context.Context, id, actor string, version int, target *string) (*AssignmentChange, error) {
	r.calls++
	r.target = target
	return &AssignmentChange{ApplicationID: id, AssignedEmployeeID: target, DataVersion: version, Status: "MANUAL"}, nil
}
func (r *workflowRepoStub) ListApplications(_ context.Context, actor, view string, f ListFilter) (*ApplicationPage, error) {
	r.calls++
	r.filter = f
	return &ApplicationPage{Items: []ApplicationListItem{}, Limit: f.Limit, Offset: f.Offset}, nil
}
func (r *workflowRepoStub) ListDecisions(_ context.Context, actor string, f ListFilter) (*DecisionPage, error) {
	r.calls++
	r.filter = f
	return &DecisionPage{Items: []DecisionListItem{}, Limit: f.Limit, Offset: f.Offset}, nil
}

const workflowTestID = "11111111-1111-4111-8111-111111111111"
const workflowTestKey = "test-key-for-bank-workflow-at-least-32-bytes"

func workflowAuth(t *testing.T) *auth.Handler {
	t.Helper()
	service, err := auth.NewService(&unusedAuthRepo{}, workflowTestKey)
	if err != nil {
		t.Fatal(err)
	}
	return auth.NewHandler(service)
}
func workflowToken(t *testing.T, role string) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": workflowTestID, "role": role, "iss": "alphahub-api", "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte(workflowTestKey))
	if err != nil {
		t.Fatal(err)
	}
	return token
}
func TestBankAdditionalOperations(t *testing.T) {
	cases := []struct {
		name, method, path, role, body string
		status, calls                  int
	}{
		{"info create", "POST", "/bank/applications/" + workflowTestID + "/info-requests", "BANK_EMPLOYEE", `{"data_version":2,"description":" Statement ","required_items":[" PDF "]}`, 201, 1},
		{"info anonymous", "POST", "/bank/applications/" + workflowTestID + "/info-requests", "", `{}`, 401, 0},
		{"info client", "POST", "/bank/applications/" + workflowTestID + "/info-requests", "CLIENT", `{}`, 403, 0},
		{"info duplicates", "POST", "/bank/applications/" + workflowTestID + "/info-requests", "BANK_EMPLOYEE", `{"data_version":2,"description":"Statement","required_items":["PDF"," PDF "]}`, 400, 0},
		{"release explicit null", "POST", "/bank/applications/" + workflowTestID + "/assignment", "BANK_EMPLOYEE", `{"data_version":2,"employee_id":null}`, 200, 1},
		{"release missing target", "POST", "/bank/applications/" + workflowTestID + "/assignment", "BANK_EMPLOYEE", `{"data_version":2}`, 400, 0},
		{"transfer invalid target", "POST", "/bank/applications/" + workflowTestID + "/assignment", "BANK_EMPLOYEE", `{"data_version":2,"employee_id":"bad"}`, 400, 0},
		{"page defaults", "GET", "/bank/applications/page?assigned=mine", "BANK_EMPLOYEE", "", 200, 1},
		{"archive", "GET", "/bank/archive?status=OFFER_EXPIRED&limit=10&offset=2", "BANK_EMPLOYEE", "", 200, 1},
		{"my decisions", "GET", "/bank/decisions?q=test", "BANK_EMPLOYEE", "", 200, 1},
		{"invalid limit", "GET", "/bank/applications/page?limit=101", "BANK_EMPLOYEE", "", 400, 0},
		{"invalid offset", "GET", "/bank/applications/page?offset=-1", "BANK_EMPLOYEE", "", 400, 0},
		{"invalid status", "GET", "/bank/archive?status=INVALID", "BANK_EMPLOYEE", "", 400, 0},
		{"decisions reject foreign assignment filter", "GET", "/bank/decisions?assigned=mine", "BANK_EMPLOYEE", "", 400, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &workflowRepoStub{}
			handler := NewHandler(NewService(repo))
			a := workflowAuth(t)
			mux := http.NewServeMux()
			add := func(pattern string, handle http.HandlerFunc) {
				mux.Handle(pattern, a.RequireAuth(auth.RequireRole("BANK_EMPLOYEE")(handle)))
			}
			add("POST /bank/applications/{id}/info-requests", handler.CreateInfoRequest)
			add("POST /bank/applications/{id}/assignment", handler.ChangeAssignment)
			add("GET /bank/applications/page", handler.ListPage)
			add("GET /bank/archive", handler.Archive)
			add("GET /bank/decisions", handler.MyDecisions)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.role != "" {
				req.Header.Set("Authorization", "Bearer "+workflowToken(t, tc.role))
			}
			out := httptest.NewRecorder()
			mux.ServeHTTP(out, req)
			if out.Code != tc.status || repo.calls != tc.calls {
				t.Fatalf("status=%d calls=%d body=%s", out.Code, repo.calls, out.Body.String())
			}
			if tc.name == "release explicit null" && !strings.Contains(out.Body.String(), `"assigned_employee_id":null`) {
				t.Fatal("released assignment is not null")
			}
			if tc.name == "page defaults" && (repo.filter.Limit != 20 || repo.filter.Assigned != workflowTestID) {
				t.Fatal("incorrect defaults or mine filter")
			}
		})
	}
}

type clientWorkflowStub struct {
	application.Repository
	calls int
	err   error
}

func (r *clientWorkflowStub) GetOffers(_ context.Context, id, user string) ([]application.ClientOffer, error) {
	r.calls++
	return []application.ClientOffer{}, r.err
}
func (r *clientWorkflowStub) RespondOffer(_ context.Context, id, offer, user string, req application.OfferResponseRequest) (*application.OfferResponse, error) {
	r.calls++
	return &application.OfferResponse{ApplicationID: id, Status: "ACCEPTED"}, r.err
}
func (r *clientWorkflowStub) GetInfoRequests(_ context.Context, id, user string) ([]application.InfoRequest, error) {
	r.calls++
	return []application.InfoRequest{}, r.err
}
func (r *clientWorkflowStub) RespondInfo(_ context.Context, id, request, user string, req application.InfoResponseRequest) (*application.Application, error) {
	r.calls++
	return &application.Application{ID: id, Status: "MANUAL"}, r.err
}
func TestClientWorkflowHTTP(t *testing.T) {
	cases := []struct {
		name, path, body, role string
		repoErr                error
		status, calls          int
	}{
		{name: "accept", path: "offers/" + workflowTestID + "/respond", body: `{"data_version":2,"action":"ACCEPT"}`, role: "CLIENT", status: 200, calls: 1},
		{name: "missing action", path: "offers/" + workflowTestID + "/respond", body: `{"data_version":2}`, role: "CLIENT", status: 400},
		{name: "expired", path: "offers/" + workflowTestID + "/respond", body: `{"data_version":2,"action":"ACCEPT"}`, role: "CLIENT", repoErr: application.ErrOfferExpired, status: 409, calls: 1},
		{name: "foreign", path: "offers/" + workflowTestID + "/respond", body: `{"data_version":2,"action":"DECLINE"}`, role: "CLIENT", repoErr: application.ErrNotFound, status: 404, calls: 1},
		{name: "bank blocked", path: "offers/" + workflowTestID + "/respond", body: `{}`, role: "BANK_EMPLOYEE", status: 403},
		{name: "reply", path: "info-requests/" + workflowTestID + "/respond", body: `{"data_version":2,"response_comment":"Uploaded"}`, role: "CLIENT", status: 200, calls: 1},
		{name: "unconfirmed reply", path: "info-requests/" + workflowTestID + "/respond", body: `{"data_version":2,"response_comment":"Uploaded"}`, role: "CLIENT", repoErr: application.ErrFinancialsUnconfirmed, status: 422, calls: 1},
		{name: "empty reply", path: "info-requests/" + workflowTestID + "/respond", body: `{"data_version":2,"response_comment":" "}`, role: "CLIENT", status: 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &clientWorkflowStub{err: tc.repoErr}
			h := application.NewHandler(application.NewService(repo))
			a := workflowAuth(t)
			mux := http.NewServeMux()
			mux.Handle("POST /applications/{id}/offers/{offerID}/respond", a.RequireAuth(auth.RequireRole("CLIENT")(http.HandlerFunc(h.RespondOffer))))
			mux.Handle("POST /applications/{id}/info-requests/{requestID}/respond", a.RequireAuth(auth.RequireRole("CLIENT")(http.HandlerFunc(h.RespondInfo))))
			req := httptest.NewRequest("POST", "/applications/"+workflowTestID+"/"+tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer "+workflowToken(t, tc.role))
			out := httptest.NewRecorder()
			mux.ServeHTTP(out, req)
			if out.Code != tc.status || repo.calls != tc.calls {
				t.Fatalf("status=%d calls=%d body=%s", out.Code, repo.calls, out.Body.String())
			}
		})
	}
}

type downloadRepoStub struct {
	documents.Repository
	missing bool
}

func (r *downloadRepoStub) GetForDownload(_ context.Context, app, doc, user, role string) (*documents.DownloadMetadata, error) {
	if r.missing || app != workflowTestID || doc != workflowTestID || user != workflowTestID {
		return nil, documents.ErrDocumentNotFound
	}
	return &documents.DownloadMetadata{Document: documents.Document{ID: doc, ApplicationID: app, OriginalFilename: "отчёт.pdf", UploadedAt: time.Now()}, StorageKey: "statement.pdf"}, nil
}
func TestProtectedPDFDownload(t *testing.T) {
	storage := documents.NewLocalStorage(t.TempDir())
	if err := storage.Save(context.Background(), "statement.pdf", []byte("%PDF-test-data")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, role, rang string
		missing          bool
		status           int
	}{
		{"bank", "BANK_EMPLOYEE", "", false, 200}, {"anonymous", "", "", false, 401}, {"client", "CLIENT", "", false, 403},
		{"range", "BANK_EMPLOYEE", "bytes=0-4", false, 206}, {"missing", "BANK_EMPLOYEE", "", true, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := documents.NewHandler(documents.NewService(&downloadRepoStub{missing: tc.missing}, storage, nil))
			a := workflowAuth(t)
			mux := http.NewServeMux()
			mux.Handle("GET /bank/applications/{id}/documents/{documentID}/download", a.RequireAuth(auth.RequireRole("BANK_EMPLOYEE")(http.HandlerFunc(h.DownloadBank))))
			req := httptest.NewRequest("GET", "/bank/applications/"+workflowTestID+"/documents/"+workflowTestID+"/download", nil)
			if tc.role != "" {
				req.Header.Set("Authorization", "Bearer "+workflowToken(t, tc.role))
			}
			if tc.rang != "" {
				req.Header.Set("Range", tc.rang)
			}
			out := httptest.NewRecorder()
			mux.ServeHTTP(out, req)
			if out.Code != tc.status {
				t.Fatalf("status=%d body=%s", out.Code, out.Body.String())
			}
			if tc.status == 200 || tc.status == 206 {
				if out.Header().Get("Content-Type") != "application/pdf" || out.Header().Get("Cache-Control") != "no-store" || !strings.HasPrefix(out.Header().Get("Content-Disposition"), "inline") {
					t.Fatal("unsafe download headers")
				}
			}
			if tc.status == 206 && out.Body.String() != "%PDF-" {
				t.Fatal("wrong byte range")
			}
		})
	}
}
func TestStorageReadBoundaries(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	storage := documents.NewLocalStorage(root)
	if err := storage.Save(ctx, "ok.pdf", []byte("%PDF-test")); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"../secret", "..\\secret", "/absolute.pdf", ".", ""} {
		if _, err := storage.Read(ctx, key); err == nil {
			t.Fatalf("unsafe key accepted: %s", key)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := storage.Read(canceled, "ok.pdf"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored")
	}
	t.Run("symlink escape", func(t *testing.T) {
		outside := filepath.Join(t.TempDir(), "secret.pdf")
		if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "link.pdf")); err != nil {
			t.Skip("symlinks unavailable")
		}
		if _, err := storage.Read(ctx, "link.pdf"); err == nil {
			t.Fatal("symlink escaped storage")
		}
	})
}

type supportingDocumentRepo struct {
	documents.Repository
	saved bool
}

func (r *supportingDocumentRepo) PrepareUpload(_ context.Context, app, user string) (*documents.UploadContext, error) {
	return &documents.UploadContext{Version: 2, INN: "7700000023"}, nil
}
func (r *supportingDocumentRepo) Save(_ context.Context, doc documents.Document, user, key string, version int, extraction *dsclient.ExtractFinancialsResponse) (*documents.Document, error) {
	if extraction != nil || version != 2 || doc.DocumentType != "SUPPORTING_DOCUMENT" || doc.ProcessingStatus != "UPLOADED" {
		return nil, errors.New("supporting PDF was extracted or misclassified")
	}
	r.saved = true
	return &doc, nil
}
func TestSupportingPDFDoesNotCallDS(t *testing.T) {
	repo := &supportingDocumentRepo{}
	service := documents.NewService(repo, documents.NewLocalStorage(t.TempDir()), nil)
	doc, err := service.Upload(context.Background(), documents.UploadInput{ApplicationID: workflowTestID, UserID: workflowTestID, DocumentType: "SUPPORTING_DOCUMENT", Filename: "certificate.pdf", Data: []byte("%PDF-support")})
	if err != nil || !repo.saved || doc.ProcessingStatus != "UPLOADED" {
		t.Fatal("supporting upload failed", err)
	}
	if _, err = service.Upload(context.Background(), documents.UploadInput{ApplicationID: workflowTestID, UserID: workflowTestID, DocumentType: "UNKNOWN", Filename: "certificate.pdf", Data: []byte("%PDF-support")}); !errors.Is(err, documents.ErrInvalidInput) {
		t.Fatal("unknown document type accepted", err)
	}
}
