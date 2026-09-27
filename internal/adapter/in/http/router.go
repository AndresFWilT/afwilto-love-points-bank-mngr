package http

import (
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
)

// UseCases agrupa los casos de uso que el router necesita, inyectados desde el
// composition root.
type UseCases struct {
	Points      in.PointsUseCase
	Catalog     in.CatalogUseCase
	Redemptions in.RedemptionUseCase
}

// NewRouter arma el router HTTP con los casos de uso ya construidos.
func NewRouter(uc UseCases, corsOrigin string) http.Handler {
	h := NewHandler(uc.Points)
	catalog := NewCatalogHandler(uc.Catalog)
	redemptions := NewRedemptionHandler(uc.Redemptions)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	if corsOrigin != "" {
		r.Use(cors(corsOrigin))
	}

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/entries", h.Award)
		r.Get("/entries", h.List)
		r.Patch("/entries/{id}", h.Edit)
		r.Post("/entries/{id}/void", h.Void)
		r.Get("/balances", h.Balances)
		r.Get("/members", h.Members)

		// Fase 2
		r.Post("/catalog-items", catalog.Create)
		r.Get("/catalog-items", catalog.List)
		r.Patch("/catalog-items/{id}", catalog.Edit)
		r.Post("/redemptions", redemptions.Redeem)
		r.Get("/redemptions", redemptions.List)
		r.Post("/redemptions/{id}/void", redemptions.Void)
	})

	return r
}

// cors habilita un allowlist de orígenes conocidos (coma-separados: el frontend
// standalone y, en la plataforma AFWILTO, también el shell host). En producción el
// proxy nginx sirve todo desde el mismo origen y este middleware ni se usa.
func cors(originList string) func(http.Handler) http.Handler {
	allowed := strings.Split(originList, ",")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(allowed, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Member-Id")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
