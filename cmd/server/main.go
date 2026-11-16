package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/oxri/oxri-go/internal/app"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:" + port
	}

	assetsDir := os.Getenv("ASSETS_DIR")
	if assetsDir == "" {
		assetsDir = "assets"
	}

	handler := app.NewServer(assetsDir)

	fmt.Printf("Res Imperii Go application listening on http://%s\n", addr)
	if err := http.ListenAndServe(addr, handler); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Server failed: %v\n", err)
		os.Exit(1)
	}
}
