// Command sol-server runs the sol HTTP API.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/zaephor-bot/sol-cli/internal/api"
)

func main() {
	addr := ":" + envOr("PORT", "8080")
	log.Printf("sol-server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, api.New()))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
