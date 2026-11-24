package main

import (
	"context"
	"fmt"
	"github.com/oxri/oxri-go/internal/config"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/internal/verse"
	"github.com/oxri/oxri-go/routes"
)

func main() {
	_ = config.LoadSecrets("db.env", "pw.env")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		if os.Getenv("RENDER") != "" || os.Getenv("APP_ENV") == "prod" {
			addr = "0.0.0.0:" + port
		} else {
			addr = "127.0.0.1:" + port
		}
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

	adminTOTPSecret := os.Getenv("ADMIN_TOTP_SECRET")
	adminPasswordHash := os.Getenv("ADMIN_PASSWORD_HASH")
	var authSvc *auth.Service
	if adminTOTPSecret != "" {
		authOpts := auth.ServiceOptions{
			TOTPSecret:   adminTOTPSecret,
			PasswordHash: adminPasswordHash,
		}
		if store != nil {
			authOpts.ReplayStore = store
		}
		var err error
		authSvc, err = auth.NewServiceWithOptions(authOpts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to initialize auth service: %v\n", err)
		} else if authSvc.HasPassword() {
			fmt.Println("Res Imperii 2FA enabled (Argon2id Password + RFC 6238 TOTP)")
		} else {
			fmt.Println("Res Imperii TOTP authentication enabled")
		}
	} else {
		fmt.Println("Notice: ADMIN_TOTP_SECRET not configured; running in read-only public mode.")
	}

	handler := routes.NewRouter(routes.Config{
		AuthService: authSvc,
		Store:       store,
		Tweets:      routes.NewTweetFiller(store, &http.Client{Timeout: 10 * time.Second}),
		Passages:    verse.NewSource(&http.Client{Timeout: 10 * time.Second}),
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
