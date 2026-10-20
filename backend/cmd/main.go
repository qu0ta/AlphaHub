package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/qu0ta/AlphaHub/internal/auth"
	"github.com/qu0ta/AlphaHub/internal/platform/db"
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
	authRepo := auth.NewPostgresRepository(pool)
	authService, err := auth.NewService(authRepo, os.Getenv("JWT_SECRET"))
	if err != nil {
		log.Fatal("Auth initialization failed: ", err)
	}

	authHandler := auth.NewHandler(authService)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("GET /api/v1/auth/me", authHandler.Me)

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
