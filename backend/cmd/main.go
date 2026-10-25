package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/qu0ta/AlphaHub/internal/application"
	"github.com/qu0ta/AlphaHub/internal/auth"
	"github.com/qu0ta/AlphaHub/internal/bank"
	"github.com/qu0ta/AlphaHub/internal/company"
	"github.com/qu0ta/AlphaHub/internal/documents"
	"github.com/qu0ta/AlphaHub/internal/dsclient"
	"github.com/qu0ta/AlphaHub/internal/financials"
	"github.com/qu0ta/AlphaHub/internal/platform/db"
	"github.com/qu0ta/AlphaHub/internal/product"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("No .env file, using environment variables")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	pool, err := db.NewPostgres(context.Background(), databaseURL)
	if err != nil {
		log.Fatal("PostgreSQL connection failed: ", err)
	}
	defer pool.Close()
	log.Println("PostgreSQL connected")

	dsClient, err := dsclient.NewClient(os.Getenv("DS_BASE_URL"))
	if err != nil {
		log.Fatalf("Init DS client: %v", err)
	}
	mux := http.NewServeMux()

	authRepo := auth.NewPostgresRepository(pool)
	authService, err := auth.NewService(authRepo, os.Getenv("JWT_SECRET"))
	if err != nil {
		log.Fatal("Auth initialization failed: ", err)
	}
	authHandler := auth.NewHandler(authService)

	clientOnly := func(handler http.HandlerFunc) http.Handler {
		return authHandler.RequireAuth(
			auth.RequireRole("CLIENT")(handler),
		)
	}
	bankOnly := func(handler http.HandlerFunc) http.Handler {
		return authHandler.RequireAuth(
			auth.RequireRole("BANK_EMPLOYEE")(handler),
		)
	}

	bankRepo := bank.NewPostgresRepository(pool)
	bankService := bank.NewService(bankRepo)
	bankHandler := bank.NewHandler(bankService)

	mux.Handle(
		"GET /api/v1/bank/applications",
		bankOnly(bankHandler.GetManual),
	)

	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("GET /api/v1/auth/me", authHandler.Me)

	productRepo := product.NewPostgresRepository(pool)
	productService := product.NewService(productRepo)
	productHandler := product.NewHandler(productService)

	mux.HandleFunc("GET /api/v1/products", productHandler.GetAll)
	mux.HandleFunc("GET /api/v1/products/{code}", productHandler.GetByCode)

	applicationRepo := application.NewPostgresRepository(pool, dsClient)
	applicationService := application.NewService(applicationRepo)
	applicationHandler := application.NewHandler(applicationService)

	mux.Handle(
		"POST /api/v1/applications",
		clientOnly(applicationHandler.Create),
	)
	mux.Handle(
		"GET /api/v1/applications",
		clientOnly(applicationHandler.List),
	)
	mux.Handle(
		"GET /api/v1/applications/{id}",
		clientOnly(applicationHandler.GetByID),
	)
	mux.Handle(
		"PATCH /api/v1/applications/{id}",
		clientOnly(applicationHandler.Update),
	)
	mux.Handle(
		"POST /api/v1/applications/{id}/submit",
		clientOnly(applicationHandler.Submit),
	)

	mux.Handle("GET /api/v1/applications/{id}/scoring", clientOnly(applicationHandler.GetScoring))

	companyRepo := company.NewRepository(pool)
	companyService := company.NewService(companyRepo)
	companyHandler := company.NewHandler(companyService)

	mux.Handle(
		"POST /api/v1/company",
		clientOnly(companyHandler.Create),
	)

	mux.Handle(
		"GET /api/v1/company/me",
		clientOnly(companyHandler.GetMe),
	)
	mux.Handle(
		"GET /api/v1/company/by-inn/{inn}",
		clientOnly(companyHandler.GetByINN),
	)

	mux.Handle(
		"POST /api/v1/company/invitations",
		clientOnly(companyHandler.CreateInvitation),
	)

	mux.Handle(
		"POST /api/v1/company/join",
		clientOnly(companyHandler.Join),
	)
	documentRepo := documents.NewRepository(pool)
	documentStorage := documents.NewLocalStorage("uploads")
	documentService := documents.NewService(
		documentRepo,
		documentStorage,
		dsClient,
	)
	documentHandler := documents.NewHandler(documentService)

	mux.Handle(
		"POST /api/v1/applications/{id}/documents",
		clientOnly(documentHandler.Upload),
	)

	mux.Handle(
		"GET /api/v1/applications/{id}/documents",
		clientOnly(documentHandler.List),
	)
	financialRepo := financials.NewRepository(pool)
	financialService := financials.NewService(financialRepo)
	financialHandler := financials.NewHandler(financialService)

	mux.Handle(
		"GET /api/v1/applications/{id}/financials",
		clientOnly(financialHandler.Get),
	)

	mux.Handle(
		"PUT /api/v1/applications/{id}/financials",
		clientOnly(financialHandler.Update),
	)

	mux.Handle(
		"POST /api/v1/applications/{id}/financials/confirm",
		clientOnly(financialHandler.Confirm),
	)
	//mux.Handle(
	//	"GET /api/v1/client/test",
	//	authHandler.RequireAuth(
	//		auth.RequireRole("CLIENT")(
	//			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	//				user, _ := auth.UserFromContext(r.Context())
	//
	//				w.Header().Set("Content-Type", "application/json")
	//				_ = json.NewEncoder(w).Encode(map[string]string{
	//					"message": "Client access granted",
	//					"user_id": user.ID,
	//					"role":    user.Role,
	//				})
	//			}),
	//		),
	//	),
	//)
	//mux.Handle(
	//	"GET /api/v1/bank/test",
	//	authHandler.RequireAuth(
	//		auth.RequireRole("BANK_EMPLOYEE")(
	//			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	//				user, _ := auth.UserFromContext(r.Context())
	//
	//				w.Header().Set("Content-Type", "application/json")
	//				_ = json.NewEncoder(w).Encode(map[string]string{
	//					"message": "Bank employee access granted",
	//					"user_id": user.ID,
	//					"role":    user.Role,
	//				})
	//			}),
	//		),
	//	),
	//)

	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("Server started on http://localhost:%s", port)

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
