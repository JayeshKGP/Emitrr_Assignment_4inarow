package main

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/jayeshdeshmukh/connect-four-backend/internal/handlers"
	"github.com/jayeshdeshmukh/connect-four-backend/internal/hub"
)

func main() {
	// Load .env file
	loadEnv(".env")

	log.Println("FourSync Game Server")
	log.Println("Starting server on http://localhost:8080")

	// Initialize hub and handlers
	gameHub := hub.NewHub()
	handler := handlers.NewHandler(gameHub)

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handler.HandleWebSocket)
	mux.HandleFunc("/health", handler.HealthCheck)
	mux.HandleFunc("/api/leaderboard", handler.GetLeaderboard)

	// CORS middleware
	corsHandler := corsMiddleware(mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Fatal(http.ListenAndServe(":"+port, corsHandler))
}

// loadEnv loads environment variables from a file
func loadEnv(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		log.Printf("No .env file found, using system environment variables")
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			os.Setenv(key, value)
		}
	}
	log.Println("Loaded environment variables from .env")
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
