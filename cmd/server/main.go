package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/routes"
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
	_ = db.LoadEnvFiles(envFiles...)

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

	assetsDir := os.Getenv("ASSETS_DIR")
	if assetsDir == "" {
		assetsDir = "assets"
	}

	adminPubKey := os.Getenv("ADMIN_SSH_PUBKEY")
	hostKeyPath := os.Getenv("SSH_HOST_KEY_PATH")
	if hostKeyPath == "" {
		hostKeyPath = resolveDefaultHostKeyPath()
	}

	var authSvc *auth.Service
	if adminPubKey != "" {
		var err error
		authSvc, err = auth.NewService(adminPubKey, hostKeyPath, sshAddr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to initialize auth service: %v\n", err)
		} else {
			if err := authSvc.StartSSHListener(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to start SSH listener on %s: %v\n", sshAddr, err)
			} else {
				fmt.Printf("Res Imperii SSH auth gateway listening on %s\n", sshAddr)
			}
		}
	} else {
		fmt.Println("Notice: ADMIN_SSH_PUBKEY not configured; running in read-only public mode.")
	}

	var store *db.Store
	dbURL := os.Getenv("TURSO_DATABASE_URL")
	dbToken := os.Getenv("TURSO_AUTH_TOKEN")
	if dbURL != "" {
		dbCtx, dbCancel := context.WithTimeout(context.Background(), 15*time.Second)
		var err error
		store, err = db.Open(dbCtx, dbURL, dbToken)
		if err != nil {
			dbCancel()
			fmt.Fprintf(os.Stderr, "Error: failed to connect to remote Turso database: %v\n", err)
			os.Exit(1)
		}
		used, _ := store.UsedStorageBytes(dbCtx)
		schemaVer, _ := store.SchemaVersion(dbCtx)
		dbCancel()
		fmt.Printf("Connected to remote Turso database (schema v%d, used %d / %d bytes)\n",
			schemaVer, used, store.MaxStorageBytes())
	} else {
		fmt.Println("Notice: TURSO_DATABASE_URL not configured.")
	}

	handler := routes.NewRouter(routes.Config{
		AssetsDir:   assetsDir,
		AuthService: authSvc,
		Store:       store,
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		fmt.Printf("Res Imperii HTTP server listening on http://%s\n", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down Res Imperii server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if authSvc != nil {
		_ = authSvc.Close()
	}
	if store != nil {
		_ = store.Close()
	}
	_ = server.Shutdown(ctx)
	fmt.Println("Server gracefully stopped.")
}
