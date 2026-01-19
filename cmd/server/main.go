package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"farm-search/internal/api"
	"farm-search/internal/db"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 8080, "Port to listen on")
	dbPath := flag.String("db", "", "Path to SQLite database")
	flag.Parse()

	// Determine paths
	execPath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	baseDir := filepath.Dir(filepath.Dir(execPath))

	// Default database path
	if *dbPath == "" {
		*dbPath = filepath.Join(baseDir, "data", "farm-search.db")
	}

	// For development, use relative paths
	if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
		// Try relative path from current working directory
		cwd, _ := os.Getwd()
		*dbPath = filepath.Join(cwd, "data", "farm-search.db")
	}

	// Static files directory
	staticDir := filepath.Join(baseDir, "web", "static")
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		cwd, _ := os.Getwd()
		staticDir = filepath.Join(cwd, "web", "static")
	}

	log.Printf("Database path: %s", *dbPath)
	log.Printf("Static files: %s", staticDir)

	// Initialize database
	database, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Create router
	router := api.NewRouter(database, staticDir)

	// Start server
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting server on http://localhost%s", addr)
	
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
