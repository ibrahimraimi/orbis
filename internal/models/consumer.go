package models

import "time"

// Consumer represents a distinct client allowed to access the API Gateway.
type Consumer struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	APIKeyHash string    `json:"api_key_hash"` // Internal use, synced to Gateway
	CustomRate float64   `json:"custom_rate,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// ConsumerResponse is used to return a consumer back to the client
// with the raw, unhashed API Key exactly once upon creation.
type ConsumerResponse struct {
	Consumer
	RawAPIKey string `json:"api_key,omitempty"` // Only populated on creation
}
