//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/magefile/mage/sh"
)

// Default target to run when no argument is given.
var Default = Run

// envWithCgoDisabled ensures CGO_ENABLED=0 is strictly enforced across all builds and tests.
func envWithCgoDisabled() map[string]string {
	return map[string]string{
		"CGO_ENABLED": "0",
	}
}

// Build compiles the application static binary into bin/ with CGO disabled.
func Build() error {
	fmt.Println("==> Compiling server binary (CGO_ENABLED=0) into bin/...")
	if err := os.MkdirAll("bin", 0755); err != nil {
		return err
	}
	binName := "bin/server"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	return sh.RunWith(envWithCgoDisabled(), "go", "build", "-trimpath", "-o", binName, "./cmd/server")
}

// Icons redraws the app icons (cmd/icongen) into assets/icons/.
func Icons() error {
	fmt.Println("==> Drawing app icons (cmd/icongen)...")
	return sh.RunWithV(envWithCgoDisabled(), "go", "run", "./cmd/icongen", "assets/icons")
}

// Run starts the development server with CGO disabled.
func Run() error {
	fmt.Println("==> Starting Res Imperii server (CGO_ENABLED=0)...")
	return sh.RunWithV(envWithCgoDisabled(), "go", "run", "./cmd/server")
}

// Test executes the full test suite with CGO disabled.
func Test() error {
	fmt.Println("==> Running tests (CGO_ENABLED=0)...")
	return sh.RunWithV(envWithCgoDisabled(), "go", "test", "-v", "./...")
}

// Typecheck type-checks the site's JavaScript (JSDoc types, strict) with
// TypeScript. Nothing is compiled; see tsconfig.json. Needs Node: run
// `npm ci` once to install the pinned TypeScript.
func Typecheck() error {
	fmt.Println("==> Type-checking assets/js (tsc --noEmit)...")
	return sh.RunV("npx", "--no-install", "tsc", "-p", ".")
}

// Check validates formatting, tests, the JavaScript types, and static builds.
func Check() error {
	if err := Test(); err != nil {
		return err
	}
	if err := Typecheck(); err != nil {
		return err
	}
	return Build()
}

// Totp generates (or preserves) ADMIN_TOTP_SECRET in ../oxri-parent/db.env
// and writes a scannable QR code PNG to ../oxri-parent/totp-qr.png.
func Totp() error {
	fmt.Println("==> Preparing TOTP seed and QR code in parent secrets folder...")
	return sh.RunWithV(envWithCgoDisabled(), "go", "run", "./cmd/totpgen")
}

// Password starts a one-time loopback website (http://127.0.0.1:8089) that hashes
// your chosen password with Argon2id, saves ADMIN_PASSWORD_HASH to ../oxri-parent/pw.env,
// and shuts itself down automatically.
func Password() error {
	fmt.Println("==> Starting one-time local password hash generator...")
	return sh.RunWithV(envWithCgoDisabled(), "go", "run", "./cmd/passgen")
}

// Backup exports journals, books, and passkey metadata from Turso into
// ../oxri-parent/backups/oxri-backup-YYYYMMDD-HHMMSS.json (0600 permissions).
func Backup() error {
	fmt.Println("==> Exporting offline Turso backup snapshot...")
	return sh.RunWithV(envWithCgoDisabled(), "go", "run", "./cmd/backup")
}

// Clean removes compiled binaries in bin/.
func Clean() error {
	fmt.Println("==> Removing bin/...")
	return os.RemoveAll("bin")
}
