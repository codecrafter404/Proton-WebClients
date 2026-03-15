package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/codecrafter404/Proton-WebClients/backend/auth"
	"github.com/codecrafter404/Proton-WebClients/backend/handlers"
	"github.com/codecrafter404/Proton-WebClients/backend/router"
	"github.com/codecrafter404/Proton-WebClients/backend/store"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "calendar.db", "SQLite database path")
	flag.Parse()

	s, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer s.Close()

	authMgr := auth.NewManager()
	h := handlers.New(s)
	mux := router.New(h, authMgr)

	// Wrap with auth middleware then CORS
	handler := authMgr.Middleware(mux)
	handler = corsMiddleware(handler)

	// Serve static test page at /static/
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/", handler)

	fmt.Printf("Calendar API server listening on %s\n", *addr)
	fmt.Println("Default credentials: proton / proton")
	fmt.Println("Login: POST /core/v4/auth {\"Username\":\"proton\",\"Password\":\"proton\"}")
	fmt.Println("Test UI: http://localhost" + *addr + "/static/test.html")
	log.Fatal(http.ListenAndServe(*addr, nil))
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
