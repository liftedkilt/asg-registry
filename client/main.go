package main

import (
	"bytes"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Server configuration
const (
	ServerBaseURL      = "http://localhost:8080"
	RegisterEndpoint   = "/allocate"
	LivenessEndpoint   = "/liveness"
	ClientInterval     = time.Millisecond * 10 // 100 clients per second
	LivenessInterval   = time.Second * 10      // 10-second liveness interval per client
	ExpirationChance   = 0.02                  // 10% chance a client will expire
	TotalClients       = 1200                  // Total clients to simulate
	SimulationDuration = 5 * time.Minute
)

// Client represents a simulated client
type Client struct {
	ID         string
	Identifier string
	LastSeen   time.Time
}

// API Request Structures
type AllocateRequest struct {
	ClientID string `json:"client_id"`
}

type AllocateResponse struct {
	Identifier string `json:"identifier"`
}

type LivenessRequest struct {
	ClientID   string `json:"client_id"`
	Identifier string `json:"identifier"`
}

// Simulated clients pool
var clients = make(map[string]*Client)
var mu sync.Mutex

func main() {
	log.Println("Starting Identifier Server Test Client...")

	stop := time.After(SimulationDuration)
	ticker := time.NewTicker(ClientInterval)

	for {
		select {
		case <-stop:
			log.Println("Simulation complete.")
			return
		case <-ticker.C:
			simulateClients()
		}
	}
}

func simulateClients() {
	mu.Lock()
	defer mu.Unlock()

	// Chance to register a new client
	if len(clients) < TotalClients {
		if rand.Float64() < 0.8 { // 80% chance to add a new client
			registerClient()
		}
	}

	// Update existing clients
	now := time.Now()
	for id, client := range clients {
		// Chance for expiration
		if rand.Float64() < ExpirationChance {
			log.Printf("Client %s (Identifier: %s) is letting their identifier expire", client.ID, client.Identifier)
			delete(clients, id)
			continue
		}

		// Send liveness if 10 seconds have passed since the last probe
		if now.Sub(client.LastSeen) >= LivenessInterval {
			sendLiveness(client)
		}
	}
}

func registerClient() {
	clientID := uuid.New().String()
	reqBody, _ := json.Marshal(AllocateRequest{ClientID: clientID})

	resp, err := http.Post(ServerBaseURL+RegisterEndpoint, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("Failed to register client %s: %v", clientID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to register client %s: HTTP %d", clientID, resp.StatusCode)
		return
	}

	var res AllocateResponse
	json.NewDecoder(resp.Body).Decode(&res)

	clients[clientID] = &Client{
		ID:         clientID,
		Identifier: res.Identifier,
		LastSeen:   time.Now().Add(-LivenessInterval), // Trigger immediate liveness on first tick
	}

	log.Printf("Registered new client: %s with Identifier: %s", clientID, res.Identifier)
}

func sendLiveness(client *Client) {
	reqBody, _ := json.Marshal(LivenessRequest{
		ClientID:   client.ID,
		Identifier: client.Identifier,
	})

	resp, err := http.Post(ServerBaseURL+LivenessEndpoint, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("Failed to send liveness for client %s: %v", client.ID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to send liveness for client %s: HTTP %d", client.ID, resp.StatusCode)
		delete(clients, client.ID)
		return
	}

	client.LastSeen = time.Now()
	log.Printf("Sent liveness for client %s (Identifier: %s)", client.ID, client.Identifier)
}
