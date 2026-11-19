package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/oxri/oxri-go/internal/app"
	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
)

func main() {
	if err := db.LoadEnvFiles("db.env", ".env"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning loading env files: %v\n", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:" + port
	}

	sshPort := os.Getenv("SSH_PORT")
	if sshPort == "" {
		sshPort = "2222"
	}
	sshAddr := os.Getenv("SSH_ADDR")
	if sshAddr == "" {
		sshAddr = "127.0.0.1:" + sshPort
	}

	adminPubKey := os.Getenv("ADMIN_SSH_PUBKEY")
	hostKeyPath := os.Getenv("HOST_KEY_PATH")
	if hostKeyPath == "" {
		hostKeyPath = ".ssh_host_key"
	}

	authSvc, err := auth.NewService(adminPubKey, hostKeyPath, sshAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize auth service: %v\n", err)
		os.Exit(1)
	}

	// Start embedded SSH loopback listener
	go func() {
		if err := authSvc.StartSSH(); err != nil && !errors.Is(err, net.ErrClosed) {
			fmt.Fprintf(os.Stderr, "SSH auth server error: %v\n", err)
		}
	}()
	defer authSvc.StopSSH()

	dbURL := os.Getenv("TURSO_DATABASE_URL")
	dbToken := os.Getenv("TURSO_AUTH_TOKEN")
	initCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	store, err := db.Open(initCtx, dbURL, dbToken)
	cancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize database store: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	assetsDir := os.Getenv("ASSETS_DIR")
	if assetsDir == "" {
		assetsDir = "assets"
	}

	handler := app.NewServer(assetsDir, authSvc, store)

	fmt.Printf("Res Imperii HTTP server listening on http://%s\n", addr)
	fmt.Printf("Res Imperii SSH auth gateway listening on %s\n", authSvc.SSHAddr())
	if dbURL == "" {
		fmt.Println("Database connected: file:res_imperii.db")
	} else {
		fmt.Printf("Database connected: %s\n", dbURL)
	}
	fmt.Printf("Authorized SSH key: %s\n", authSvc.AuthorizedKeyComment())

	if err := http.ListenAndServe(addr, handler); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "HTTP server failed: %v\n", err)
		os.Exit(1)
	}
}
