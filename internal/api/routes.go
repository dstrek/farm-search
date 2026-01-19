package api

import (
	"farm-search/internal/db"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// NewRouter creates and configures the Chi router
func NewRouter(database *db.DB, staticDir string) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(Logger)
	r.Use(CORS)

	// Create handlers
	h := NewHandlers(database)

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/properties", h.ListProperties)
		r.Get("/properties/{id}", h.GetProperty)
		r.Get("/filters/options", h.GetFilterOptions)
		r.Post("/scrape/trigger", h.TriggerScrape)
	})

	// Serve static files
	fileServer := http.FileServer(http.Dir(staticDir))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// Serve isochrone data
	isochroneServer := http.FileServer(http.Dir(staticDir + "/data/isochrones"))
	r.Handle("/data/isochrones/*", http.StripPrefix("/data/isochrones/", isochroneServer))

	// Serve index.html for root
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, staticDir+"/../templates/index.html")
	})

	return r
}
