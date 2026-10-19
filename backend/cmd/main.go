package main

import (
	"log"
	"net/http"

	"github.com/qu0ta/AlphaHub/internal/handler"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handler.Health)
	log.Println("Server started on http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}

}
