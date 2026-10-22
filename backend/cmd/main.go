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

	//dsClient, err := dsclient.NewClient(os.Getenv("DS_BASE_URL"))
	//if err != nil {
	//	log.Fatalf("Init DS client: %v", err)
	//}
	//_ = dsClient

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

	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("GET /api/v1/auth/me", authHandler.Me)

	productRepo := product.NewPostgresRepository(pool)
	productService := product.NewService(productRepo)
	productHandler := product.NewHandler(productService)

	mux.HandleFunc("GET /api/v1/products", productHandler.GetAll)
	mux.HandleFunc("GET /api/v1/products/{code}", productHandler.GetByCode)

	applicationRepo := application.NewPostgresRepository(pool)
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
