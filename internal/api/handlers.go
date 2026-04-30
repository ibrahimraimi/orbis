package api

import (
	"encoding/json"
	"net/http"
	"orbis/internal/models"
	"orbis/internal/registry"

	"github.com/go-chi/chi/v5"
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
