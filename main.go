package main

import (
	"fhirpath-lab-go-server/internal/server"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	mux := server.NewMux()
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
	log.Printf("fhirpath-lab-go-server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
