package main

import (
	"log"
	"net/http"
)

var config *Config

func main() {
	var err error
	config, err = LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize Database
	initDB()

	defer db.Close()

	// Preload Identifiers
	preloadIdentifiers()

	// HTTP Handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/allocate", allocateHandler)
	mux.HandleFunc("/client/{client_id}", clientDetailsHandler)
	mux.HandleFunc("/identifier/{identifier}", identifierDetailsHandler)
	mux.HandleFunc("/allocated", allocatedHandler)
	mux.HandleFunc("/identifiers", identifiersHandler)
	mux.HandleFunc("/liveness", livenessHandler)
	mux.HandleFunc("/release", releaseHandler)
	mux.HandleFunc("/stats", statsHandler)

	// Start HTTP Server
	server := &http.Server{
		Addr:         config.Server.Address,
		Handler:      mux,
		IdleTimeout:  config.Server.IdleTimeout,
		ReadTimeout:  config.Server.ReadTimeout,
		WriteTimeout: config.Server.WriteTimeout,
	}

	go releaseStaleIdentifiers()

	log.Printf("Server started on %s", config.Server.Address)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %s", err)
	}
}
