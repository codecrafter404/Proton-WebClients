package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/codecrafter404/Proton-WebClients/backend/auth"
	"github.com/codecrafter404/Proton-WebClients/backend/handlers"
	"github.com/codecrafter404/Proton-WebClients/backend/router"
	"github.com/codecrafter404/Proton-WebClients/backend/store"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "calendar.db", "SQLite database path")
	webDir := flag.String("web", "web", "directory containing the built frontend (index.html + assets)")
	flag.Parse()

	s, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer s.Close()

	authMgr, err := auth.NewManager()
	if err != nil {
		log.Fatalf("failed to create auth manager: %v", err)
	}

	h := handlers.New(s)
	mux := router.New(h, authMgr)

	// Wrap with auth middleware then CORS
	apiHandler := authMgr.Middleware(mux)
	apiHandler = corsMiddleware(apiHandler)

	// Serve static test page at /static/
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// API routes under /api/ prefix (strip prefix before routing)
	http.Handle("/api/", http.StripPrefix("/api", apiHandler))

	// Also serve API routes without prefix for backwards compatibility
	http.Handle("/calendar/", apiHandler)
	http.Handle("/core/", apiHandler)
	http.Handle("/auth/", apiHandler)
	http.Handle("/settings/", apiHandler)

	// Serve the built Proton Calendar frontend as SPA
	if info, err := os.Stat(*webDir); err == nil && info.IsDir() {
		spa := spaHandler{root: os.DirFS(*webDir), dir: *webDir}
		http.Handle("/", spa)
		fmt.Printf("Serving Proton Calendar frontend from %s\n", *webDir)
	} else {
		// Fallback: serve a simple redirect to the static calendar page
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				http.Redirect(w, r, "/static/calendar.html", http.StatusFound)
				return
			}
			http.NotFound(w, r)
		})
		fmt.Printf("No frontend build found at %s, using static calendar page\n", *webDir)
	}

	fmt.Printf("Calendar API server listening on %s\n", *addr)
	fmt.Println("Default credentials: proton / proton (SRP authenticated)")
	fmt.Printf("Web UI:     http://localhost%s/\n", *addr)
	fmt.Printf("API:        http://localhost%s/api/\n", *addr)
	fmt.Printf("Static UI:  http://localhost%s/static/calendar.html\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

// spaHandler serves a single-page application: static files if they exist,
// otherwise falls back to index.html for client-side routing.
type spaHandler struct {
	root fs.FS
	dir  string
}

func (s spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	// Try to serve the exact file
	if _, err := fs.Stat(s.root, path); err == nil {
		http.FileServer(http.Dir(s.dir)).ServeHTTP(w, r)
		return
	}

	// Fall back to index.html for SPA routing
	r.URL.Path = "/"
	http.FileServer(http.Dir(s.dir)).ServeHTTP(w, r)
}

// corsMiddleware adds CORS headers to allow the frontend to communicate.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, x-pm-uid, x-pm-appversion, x-pm-locale")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
