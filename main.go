package main

import (
	"fhirpath-lab-go/internal"
	"flag"
	"github.com/damedic/fhir-toolbox-go/model"
	"github.com/damedic/fhir-toolbox-go/rest"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	var addrFlag string
	flag.StringVar(&addrFlag, "addr", "", "listen address, e.g. :3001 or 127.0.0.1:3001 (overrides PORT)")
	flag.Parse()

	addr := ":3001"
	if strings.TrimSpace(addrFlag) != "" {
		addr = addrFlag
	} else if v := os.Getenv("PORT"); strings.TrimSpace(v) != "" {
		if strings.HasPrefix(v, ":") || strings.Contains(v, ":") {
			addr = v
		} else {
			addr = ":" + v
		}
	}

	backend := &internal.Backend{BaseURL: addr}
	server := withCORS(&rest.Server[model.R4]{Backend: backend})

	log.Printf("fhirpath-lab-go-cmd listening on %s", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatal(err)
	}
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
