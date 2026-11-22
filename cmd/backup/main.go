package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/oxri/oxri-go/internal/db"
)

func resolveSecretsDir() string {
	parentDir := filepath.Join("..", "oxri-parent")
	if info, err := os.Stat(parentDir); err == nil && info.IsDir() {
		return parentDir
	}
	return ".."
}

func defaultBackupPath() string {
	ts := time.Now().UTC().Format("20060102-150405")
	return filepath.Join(resolveSecretsDir(), "backups", "oxri-backup-"+ts+".json")
}

func main() {
	outPath := flag.String("out", "", "Output JSON path (defaults to ../oxri-parent/backups/oxri-backup-YYYYMMDD-HHMMSS.json)")
	restorePath := flag.String("restore", "", "Path to a JSON snapshot file to restore into Turso")
	flag.Parse()

	_ = db.LoadEnvFiles(
		filepath.Join("..", "oxri-parent", "db.env"),
		filepath.Join("..", "oxri-parent", ".env"),
		filepath.Join("..", "db.env"),
		filepath.Join("..", ".env"),
	)

	dbURL := os.Getenv("TURSO_DATABASE_URL")
	dbToken := os.Getenv("TURSO_AUTH_TOKEN")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "Error: TURSO_DATABASE_URL is not configured (checked environment and ../oxri-parent/db.env)")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store, err := db.Open(ctx, dbURL, dbToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open Turso database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = store.Close() }()

	if *restorePath != "" {
		raw, err := os.ReadFile(*restorePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to read snapshot file: %v\n", err)
			os.Exit(1)
		}
		var snap db.Snapshot
		if err := json.Unmarshal(raw, &snap); err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid snapshot JSON: %v\n", err)
			os.Exit(1)
		}
		if err := store.RestoreSnapshot(ctx, &snap); err != nil {
			fmt.Fprintf(os.Stderr, "Error: restore failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Restored snapshot (%d journals, %d books, %d passkeys) from:\n  %s\n",
			len(snap.Journals), len(snap.Books), len(snap.Passkeys), *restorePath)
		return
	}

	target := *outPath
	if target == "" {
		target = defaultBackupPath()
	}

	snap, err := store.ExportSnapshot(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: export failed: %v\n", err)
		os.Exit(1)
	}

	encoded, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: marshal snapshot failed: %v\n", err)
		os.Exit(1)
	}
	encoded = append(encoded, '\n')

	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Error: create backup directory failed: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(target, encoded, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error: write backup file failed: %v\n", err)
		os.Exit(1)
	}

	absTarget, _ := filepath.Abs(target)
	fmt.Printf("Exported snapshot (%d journals, %d books, %d passkeys) to:\n  %s\n",
		len(snap.Journals), len(snap.Books), len(snap.Passkeys), absTarget)
}
