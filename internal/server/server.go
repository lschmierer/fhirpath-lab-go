package server

import (
	"github.com/DAMEDIC/fhir-toolbox-go/model"
	"github.com/DAMEDIC/fhir-toolbox-go/rest"
	"net/http"
	"strings"
)

// NewMux creates the HTTP mux with all FHIRPath lab endpoints registered.
func NewMux() *http.ServeMux {
	mux := http.NewServeMux()
	backend := &Backend{}
	// Mount R4 server at root so operation routes like "/$fhirpath" are handled.
	r4Server := &rest.Server[model.R4]{Backend: backend}
	mux.Handle("/", withCORS(r4Server))
	return mux
}

// CORS helpers
func corsAllowedOrigin(origin string) (string, bool) {
	allowed := []string{
		"https://fhirpath-lab.com",
		"https://dev.fhirpath-lab.com",
		"http://localhost:3000",
	}
	for _, a := range allowed {
		if strings.EqualFold(origin, a) {
			return a, true
		}
	}
	return "", false
}
func writeCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if o, ok := corsAllowedOrigin(origin); ok {
		w.Header().Set("Access-Control-Allow-Origin", o)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	}
}

// withCORS wraps a handler to write CORS headers and handle OPTIONS preflight.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
