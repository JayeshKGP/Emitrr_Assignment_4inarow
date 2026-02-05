package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	fmt.Println("Connect Four Game Server")
	fmt.Println("Server starting on http://localhost:8080")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Connect Four Server is running!"))
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "ok"}`))
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
