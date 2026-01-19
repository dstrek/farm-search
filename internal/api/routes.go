package api

import (
	"farm-search/internal/db"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// Cache buster timestamp (set at startup)
var cacheBuster = strconv.FormatInt(time.Now().Unix(), 10)

// Mapbox token from environment
var mapboxToken = os.Getenv("MAPBOX_TOKEN")

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

	// Serve index.html for root with cache buster
	tmplPath := filepath.Join(staticDir, "..", "templates", "index.html")
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles(tmplPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		tmpl.Execute(w, map[string]string{
			"V":           cacheBuster,
			"MapboxToken": mapboxToken,
		})
	})

	return r
}
