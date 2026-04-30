package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"orbis/internal/models"
	"orbis/internal/registry"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	registry *registry.Registry
}

func NewHandler(reg *registry.Registry) *Handler {
	return &Handler{registry: reg}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var svc models.Service
	if err := json.NewDecoder(r.Body).Decode(&svc); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.registry.Register(&svc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(svc)
}

func (h *Handler) Deregister(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing service id", http.StatusBadRequest)
		return
	}

	if err := h.registry.Deregister(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListServices(w http.ResponseWriter, r *http.Request) {
	services := h.registry.ListServices()
	json.NewEncoder(w).Encode(services)
}

func (h *Handler) Lookup(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		http.Error(w, "missing service name", http.StatusBadRequest)
		return
	}

	services := h.registry.GetHealthyServicesByName(name)
	json.NewEncoder(w).Encode(services)
}

func (h *Handler) GetService(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing service id", http.StatusBadRequest)
		return
	}

	svc, ok := h.registry.GetService(id)
	if !ok {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(svc)
}

func (h *Handler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing service id", http.StatusBadRequest)
		return
	}

	if err := h.registry.Heartbeat(id); err != nil {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetHealth(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing service id", http.StatusBadRequest)
		return
	}

	svc, ok := h.registry.GetService(id)
	if !ok {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": string(svc.Health)})
}

// Consumers

func (h *Handler) CreateConsumer(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name       string  `json:"name"`
		CustomRate float64 `json:"custom_rate,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if payload.Name == "" {
		http.Error(w, "missing name", http.StatusBadRequest)
		return
	}

	rawKey := make([]byte, 32)
	if _, err := rand.Read(rawKey); err != nil {
		http.Error(w, "failed to generate api key", http.StatusInternalServerError)
		return
	}
	apiKeyStr := hex.EncodeToString(rawKey)

	hash := sha256.Sum256([]byte(apiKeyStr))
	hashStr := hex.EncodeToString(hash[:])

	consumer := &models.Consumer{
		ID:         uuid.New().String(),
		Name:       payload.Name,
		APIKeyHash: hashStr,
		CustomRate: payload.CustomRate,
	}

	if err := h.registry.CreateConsumer(consumer); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := models.ConsumerResponse{
		Consumer:  *consumer,
		RawAPIKey: apiKeyStr,
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) ListConsumers(w http.ResponseWriter, r *http.Request) {
	consumers := h.registry.ListConsumers()
	json.NewEncoder(w).Encode(consumers)
}

func (h *Handler) DeleteConsumer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing consumer id", http.StatusBadRequest)
		return
	}

	if err := h.registry.DeleteConsumer(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Unified Watch Stream
func (h *Handler) WatchEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	ch := h.registry.Broker.Subscribe()
	defer h.registry.Broker.Unsubscribe(ch)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-ch:
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}
