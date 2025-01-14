package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type AllocatedMapping struct {
	Identifier string    `json:"identifier"`
	LockedBy   string    `json:"locked_by"`
	LastSeen   time.Time `json:"last_seen"`
}

type AllocateRequest struct {
	ClientID string `json:"client_id"`
}

type AllocateResponse struct {
	Identifier string `json:"identifier"`
}

// Identifier represents an identifier's allocation status
type Identifier struct {
	Identifier string     `json:"identifier"`
	LockedBy   *string    `json:"locked_by,omitempty"`
	LastSeen   *time.Time `json:"last_seen,omitempty"`
	Allocated  bool       `json:"allocated"`
}

type LivenessRequest struct {
	ClientID   string `json:"client_id"`
	Identifier string `json:"identifier"`
}

func allocateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var req AllocateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ClientID == "" {
		http.Error(w, "client_id is required", http.StatusBadRequest)
		return
	}

	// Step 1: Check if the client already has an allocated identifier
	var existingIdentifier string
	err := db.QueryRow(`
		SELECT identifier 
		FROM identifiers 
		WHERE locked_by = ?`,
		req.ClientID,
	).Scan(&existingIdentifier)

	if err == nil {
		// Client already has an identifier
		log.Printf("Client %s already allocated identifier %s", req.ClientID, existingIdentifier)
		json.NewEncoder(w).Encode(AllocateResponse{Identifier: existingIdentifier})
		return
	} else if err != sql.ErrNoRows {
		// Other database error
		log.Printf("Error checking existing allocation for client %s: %v", req.ClientID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Step 2: Allocate a new identifier if none exists
	var newIdentifier string
	err = db.QueryRow(`
		UPDATE identifiers 
		SET locked_by = ?, last_seen = ?
		WHERE identifier IN (
			SELECT identifier FROM identifiers WHERE locked_by IS NULL LIMIT 1
		)
		RETURNING identifier`,
		req.ClientID, time.Now(),
	).Scan(&newIdentifier)

	if err == sql.ErrNoRows {
		// No available identifiers
		log.Printf("Allocation failed: No available identifiers for client %s", req.ClientID)
		http.Error(w, "No available identifiers", http.StatusServiceUnavailable)
		return
	} else if err != nil {
		// Other database error
		log.Printf("Error allocating identifier for client %s: %v", req.ClientID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("New identifier allocated: ClientID=%s, Identifier=%s", req.ClientID, newIdentifier)
	json.NewEncoder(w).Encode(AllocateResponse{Identifier: newIdentifier})
}

func allocatedHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	rows, err := db.Query(`
		SELECT identifier, locked_by, last_seen 
		FROM identifiers 
		WHERE locked_by IS NOT NULL
	`)
	if err != nil {
		log.Printf("Error fetching allocated mappings: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var mappings []AllocatedMapping
	for rows.Next() {
		var mapping AllocatedMapping
		var lastSeen string
		err := rows.Scan(&mapping.Identifier, &mapping.LockedBy, &lastSeen)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}

		// Parse last_seen into a time.Time object
		mapping.LastSeen, err = time.Parse(time.RFC3339, lastSeen)
		if err != nil {
			log.Printf("Error parsing last_seen: %v", err)
			continue
		}

		mappings = append(mappings, mapping)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Rows iteration error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(mappings); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func clientDetailsHandler(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("client_id")
	if clientID == "" {
		http.Error(w, "Client ID is required", http.StatusBadRequest)
		return
	}

	var identifier, lastSeen string
	err := db.QueryRow(`
		SELECT identifier, last_seen
		FROM identifiers
		WHERE locked_by = ?`,
		clientID,
	).Scan(&identifier, &lastSeen)

	if err == sql.ErrNoRows {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("Error fetching client details: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"client_id":  clientID,
		"identifier": identifier,
		"last_seen":  lastSeen,
	})
}

func identifierDetailsHandler(w http.ResponseWriter, r *http.Request) {
	// Extract identifier from the path
	identifier := r.PathValue("identifier")
	if identifier == "" {
		http.Error(w, "Identifier is required", http.StatusBadRequest)
		return
	}

	var clientID, lastSeen string
	err := db.QueryRow(`
		SELECT locked_by, last_seen
		FROM identifiers
		WHERE identifier = ?`,
		identifier,
	).Scan(&clientID, &lastSeen)

	if err == sql.ErrNoRows {
		http.Error(w, "Identifier not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("Error fetching identifier details: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"client_id":  clientID,
		"identifier": identifier,
		"last_seen":  lastSeen,
	})
}

// identifiersHandler handles listing all identifiers and their status
func identifiersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	rows, err := db.Query(`
		SELECT identifier, locked_by, last_seen 
		FROM identifiers
	`)
	if err != nil {
		log.Printf("Error fetching all identifiers: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var identifiers []Identifier
	for rows.Next() {
		var id Identifier
		var lockedBy sql.NullString
		var lastSeen sql.NullString

		err := rows.Scan(&id.Identifier, &lockedBy, &lastSeen)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}

		// Handle nullable fields
		if lockedBy.Valid {
			id.LockedBy = &lockedBy.String
			id.Allocated = true
		} else {
			id.Allocated = false
		}

		if lastSeen.Valid {
			parsedTime, err := time.Parse(time.RFC3339, lastSeen.String)
			if err == nil {
				id.LastSeen = &parsedTime
			}
		}

		identifiers = append(identifiers, id)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Rows iteration error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(identifiers); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func livenessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ClientID   string `json:"client_id"`
		Identifier string `json:"identifier"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ClientID == "" || req.Identifier == "" {
		http.Error(w, "client_id and identifier are required", http.StatusBadRequest)
		return
	}

	var dbClientID string
	err := db.QueryRow(`
		SELECT locked_by 
		FROM identifiers 
		WHERE identifier = ?`,
		req.Identifier,
	).Scan(&dbClientID)

	if err == sql.ErrNoRows {
		// Identifier does not exist
		log.Printf("Liveness probe failed: Identifier %s not found", req.Identifier)
		http.Error(w, "Identifier not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("Error querying liveness for identifier %s: %v", req.Identifier, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if dbClientID != req.ClientID {
		// ClientID does not match the current owner of the identifier
		log.Printf("Liveness probe mismatch: Identifier %s locked by %s, but %s attempted to claim it",
			req.Identifier, dbClientID, req.ClientID)

		// Respond with a clear error message
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{
			"error":       "Identifier mismatch",
			"expected_id": dbClientID,
			"your_id":     req.ClientID,
			"message":     "Your client_id does not match the current owner of this identifier. Triggering shutdown is recommended.",
		})
		return
	}

	// Update last_seen timestamp for valid liveness probe
	_, err = db.Exec(`
		UPDATE identifiers 
		SET last_seen = ?
		WHERE identifier = ? AND locked_by = ?`,
		time.Now(), req.Identifier, req.ClientID,
	)

	if err != nil {
		log.Printf("Error updating liveness for client %s: %v", req.ClientID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("Liveness updated: Identifier=%s, ClientID=%s", req.Identifier, req.ClientID)
	w.WriteHeader(http.StatusOK)
}

func releaseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ClientID   string `json:"client_id"`
		Identifier string `json:"identifier"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	_, err := db.Exec(`
		UPDATE identifiers
		SET locked_by = NULL, last_seen = NULL
		WHERE identifier = ? AND locked_by = ?`,
		req.Identifier, req.ClientID,
	)

	if err != nil {
		log.Printf("Error releasing identifier: %v", err)
		http.Error(w, "Failed to release identifier", http.StatusInternalServerError)
		return
	}

	log.Printf("Client %s manually released identifier %s", req.ClientID, req.Identifier)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Identifier released successfully",
	})
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	var total, allocated, stale int

	db.QueryRow(`SELECT COUNT(*) FROM identifiers`).Scan(&total)
	db.QueryRow(`SELECT COUNT(*) FROM identifiers WHERE locked_by IS NOT NULL`).Scan(&allocated)
	db.QueryRow(`SELECT COUNT(*) FROM identifiers WHERE last_seen < ?`, time.Now().Add(-config.Server.StaleTimeout)).Scan(&stale)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"total_identifiers":     total,
		"allocated_identifiers": allocated,
		"available_identifiers": total - allocated,
		"stale_identifiers":     stale,
	})
}
