//go:build mage
// +build mage

package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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
// TypeScript, and the lab script's (cmd/lab, which runs on Node). Nothing is
// compiled; see tsconfig.json and cmd/lab/tsconfig.json. Needs Node: run
// `npm ci` once to install the pinned TypeScript.
func Typecheck() error {
	fmt.Println("==> Type-checking assets/js and cmd/lab (tsc --noEmit)...")
	if err := sh.RunV("npx", "--no-install", "tsc", "-p", "."); err != nil {
		return err
	}
	return sh.RunV("npx", "--no-install", "tsc", "-p", "cmd/lab")
}

// Lab runs the repeatable mobile lab profile (cmd/lab, PROFILING.md) against
// the server at $LAB_BASE (default http://127.0.0.1:8080, which must be
// running signed out) and writes the report to bin/lab.md. Needs Chrome on
// the machine the script runs on: inside WSL that is the Windows host.
func Lab() error {
	base := os.Getenv("LAB_BASE")
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	fmt.Println("==> Lab: budget-phone profile, 5 cold runs per page against " + base)
	return lab("bin/lab.md", "-base", base, "-runs", "5")
}

// LabSeeded runs the lab against the seeded site (internal/labsite) with the
// CI budget, and writes bin/lab.md. The script starts the site itself
// (cmd/labserver) and measures it; no server needs to be running and no
// credentials are read. Exits 1 if a page shifts (any run's CLS over 0.1) or
// a run fails. Needs Chrome on the machine the script runs on.
func LabSeeded() error {
	fmt.Println("==> Lab: the seeded site, 3 cold runs per page, the CI budget")
	return lab("bin/lab.md", "-local", "-runs", "3", "-parallel", "4", "-budget")
}

// labNode is the Node that runs the lab script (cmd/lab/lab.mjs). The script
// drives a Chrome, so it runs where a Chrome is. Where the site is built in
// WSL (Principia #9) that is the Windows host: its Node, reached from WSL as
// node.exe through the interop, with its Chrome. Anywhere else it is the
// node on PATH (CI).
func labNode() (string, error) {
	if runtime.GOOS == "linux" {
		if release, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil && strings.Contains(strings.ToLower(string(release)), "microsoft") {
			if _, err := exec.LookPath("node.exe"); err == nil {
				return "node.exe", nil
			}
			return "", errors.New("inside WSL the lab runs on the Windows host: its Node was not found on PATH (node.exe)")
		}
	}
	if _, err := exec.LookPath("node"); err == nil {
		return "node", nil
	}
	return "", errors.New("the lab is a Node script (cmd/lab/lab.mjs): Node 18 or later is needed")
}

// lab runs the lab script with args and writes its report to report (the
// folder is made if it is missing).
func lab(report string, args ...string) error {
	node, err := labNode()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(report), 0o755); err != nil {
		return err
	}
	return sh.RunV(node, append([]string{"cmd/lab/lab.mjs", "-out", report}, args...)...)
}

// Benchmark measures the site with its real data against Google's Core Web
// Vitals (PROFILING.md): it builds the server, starts it signed out on
// 127.0.0.1:8089 against the database in db.env (reads only: a signed-out
// reader writes nothing, and local views are not recorded as field data),
// runs 10 cold budget-phone runs per page with each page's LCP broken into
// its parts, writes bin/benchmark.md, and stops the server. Needs Chrome on
// the machine the lab script runs on (labNode).
func Benchmark() error {
	if err := os.MkdirAll("bin", 0755); err != nil {
		return err
	}
	bin := "bin/server-bench"
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	fmt.Println("==> Benchmark: building the server...")
	if err := sh.RunWithV(envWithCgoDisabled(), "go", "build", "-o", bin, "./cmd/server"); err != nil {
		return err
	}
	const addr = "127.0.0.1:8089"
	server := exec.Command(bin)
	server.Env = append(withoutEnv(os.Environ(), "DEV_AUTO_LOGIN", "ADDR"), "ADDR="+addr)
	server.Stdout, server.Stderr = os.Stdout, os.Stderr
	if err := server.Start(); err != nil {
		return err
	}
	defer func() { _ = server.Process.Kill(); _, _ = server.Process.Wait() }()

	base := "http://" + addr
	deadline := time.Now().Add(45 * time.Second)
	for {
		res, err := http.Get(base + "/healthz")
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("benchmark: the server did not answer at %s", base)
		}
		time.Sleep(300 * time.Millisecond)
	}
	fmt.Println("==> Benchmark: 10 cold budget-phone runs per page against real data at " + base)
	return lab("bin/benchmark.md", "-base", base, "-runs", "10", "-breakdown", "-budget")
}

// withoutEnv drops the named variables from env.
func withoutEnv(env []string, names ...string) []string {
	var out []string
	for _, kv := range env {
		keep := true
		for _, n := range names {
			if strings.HasPrefix(kv, n+"=") {
				keep = false
			}
		}
		if keep {
			out = append(out, kv)
		}
	}
	return out
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
