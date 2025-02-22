package main

import (
	"log"
	"net/http"

	"speedtest/internal/handlers"

	"github.com/gorilla/mux"
)

func main() {
	downloadHandler := handlers.NewDownloadHandler()

	r := mux.NewRouter()
	// POST /download/init with JSON {"size_mb":10} for example
	r.HandleFunc("/download/init", downloadHandler.InitDownload).Methods("POST")
	// GET /download/data?session_id=UUID
	r.HandleFunc("/download/data", downloadHandler.DownloadData).Methods("GET")
	// POST /download/verify with JSON {"session_id":"XYZ","computed_hash":"..."}
	r.HandleFunc("/download/verify", downloadHandler.VerifyDownload).Methods("POST")
	// GET /download/speed
	r.HandleFunc("/download/speed", downloadHandler.GetSpeed).Methods("GET")

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	log.Println("Speed test server listening on :8080")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
