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

// Check validates formatting, tests, and static builds.
func Check() error {
	if err := Test(); err != nil {
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

// Clean removes compiled binaries in bin/.
func Clean() error {
	fmt.Println("==> Removing bin/...")
	return os.RemoveAll("bin")
}
