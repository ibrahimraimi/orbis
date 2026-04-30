package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewRouter(h *Handler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Handle("/metrics", promhttp.Handler())

	r.Route("/v1", func(r chi.Router) {
		r.Route("/services", func(r chi.Router) {
			r.Post("/register", h.Register)
			r.Get("/", h.ListServices)
			r.Get("/{name}", h.Lookup)
			
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", h.GetService)
				r.Delete("/deregister", h.Deregister)
				r.Put("/heartbeat", h.Heartbeat)
				r.Get("/health", h.GetHealth)
			})
		})
		
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})
	})

	return r
}
