package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/oxri/oxri-go/internal/app"
	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
)

func resolveDefaultHostKeyPath() string {
	parentDir := filepath.Join("..", "oxri-parent")
	if info, err := os.Stat(parentDir); err == nil && info.IsDir() {
		return filepath.Join(parentDir, ".ssh_host_key")
	}
	return filepath.Join("..", ".ssh_host_key")
}

func main() {
	envFiles := []string{
		filepath.Join("..", "oxri-parent", "db.env"),
		filepath.Join("..", "oxri-parent", ".env"),
		filepath.Join("..", "db.env"),
		filepath.Join("..", ".env"),
	}
	if err := db.LoadEnvFiles(envFiles...); err != nil {
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
		hostKeyPath = resolveDefaultHostKeyPath()
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
	if err == nil {
		if usedBytes, uErr := store.UsedStorageBytes(initCtx); uErr == nil && usedBytes > 0 {
			// Temporarily lock the storage limit to the exact size of the current entry for UI testing.
			store.SetMaxStorageBytes(usedBytes)
		}
	}
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
	fmt.Printf("Database connected: %s\n", dbURL)
	fmt.Printf("Storage budget limit: %d bytes\n", store.MaxStorageBytes())
	fmt.Printf("Authorized SSH key: %s\n", authSvc.AuthorizedKeyComment())

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       35 * time.Second,
		WriteTimeout:      35 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "HTTP server failed: %v\n", err)
		os.Exit(1)
	}
}
