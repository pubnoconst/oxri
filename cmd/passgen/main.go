package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/oxri/oxri-go/internal/auth"
)

func resolveSecretsDir() string {
	parentDir := filepath.Join("..", "oxri-parent")
	if info, err := os.Stat(parentDir); err == nil && info.IsDir() {
		return parentDir
	}
	return ".."
}

func upsertEnvVar(envPath, key, value string) error {
	var existingContent string
	if data, err := os.ReadFile(envPath); err == nil {
		existingContent = string(data)
	}
	lines := []string{}
	if existingContent != "" {
		lines = strings.Split(strings.ReplaceAll(existingContent, "\r\n", "\n"), "\n")
	}
	prefix := key + "="
	replaced := false
	for i, rawLine := range lines {
		if strings.HasPrefix(strings.TrimSpace(rawLine), prefix) {
			lines[i] = prefix + value
			replaced = true
			break
		}
	}
	if !replaced {
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, prefix+value)
	}
	out := strings.Join(lines, "\n") + "\n"
	if err := os.MkdirAll(filepath.Dir(envPath), 0700); err != nil {
		return err
	}
	return os.WriteFile(envPath, []byte(out), 0600)
}

func renderPage(title, body string) string {
	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + html.EscapeString(title) + `</title>
<style>
  :root { color-scheme: light dark; --page: #f5f4f0; --ink: #34332f; --card: #ecebe6; --border: rgba(52,51,47,0.18); --danger: #b54541; --ok: #3b6e44; }
  @media (prefers-color-scheme: dark) { :root { --page: #0f1113; --ink: #d6d6d3; --card: #16191c; --border: rgba(214,214,211,0.18); --danger: #e06c68; --ok: #87b382; } }
  * { box-sizing: border-box; }
  body { margin: 0; min-height: 100svh; display: grid; place-items: center; padding: 2rem 1.25rem; background: var(--page); color: var(--ink); font-family: "EB Garamond", Georgia, serif; line-height: 1.55; }
  main { width: min(30rem, 100%); padding: 2rem; border: 1px solid var(--border); border-radius: 0.5rem; background: var(--card); }
  h1 { margin: 0 0 0.35rem; font-weight: 400; font-size: 1.85rem; letter-spacing: 0.04em; }
  p.sub { margin: 0 0 1.4rem; opacity: 0.7; font-size: 1rem; }
  form { display: flex; flex-direction: column; gap: 0.85rem; }
  label { font-size: 0.92rem; opacity: 0.78; }
  input[type=password] { width: 100%; padding: 0.7rem 0.9rem; border: 1px solid var(--border); border-radius: 0.35rem; background: var(--page); color: var(--ink); font: inherit; font-size: 1.05rem; }
  button { align-self: center; margin-top: 0.4rem; padding: 0.55rem 1.4rem; border: 1px solid var(--border); border-radius: 0.35rem; background: var(--page); color: var(--ink); font: inherit; font-size: 1rem; cursor: pointer; }
  button:hover { border-color: var(--ink); }
  .err { margin: 0 0 1rem; padding: 0.65rem 0.85rem; border: 1px solid var(--danger); border-radius: 0.35rem; color: var(--danger); font-size: 0.94rem; }
  .ok { margin: 0 0 1rem; padding: 0.75rem 0.9rem; border: 1px solid var(--ok); border-radius: 0.35rem; color: var(--ok); font-size: 0.98rem; }
  code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.84rem; word-break: break-all; }
</style>
</head>
<body><main>` + body + `</main></body>
</html>`
}

func renderForm(csrfToken, envPath, errMsg string) string {
	errBlock := ""
	if errMsg != "" {
		errBlock = `<p class="err" role="alert">` + html.EscapeString(errMsg) + `</p>`
	}
	return renderPage("Res Imperii — One-Time Password Hash Setup", `
		<h1>PASSWORD SETUP</h1>
		<p class="sub">One-time local generator. Saves an Argon2id hash to <code>`+html.EscapeString(envPath)+`</code> and shuts down automatically.</p>
		`+errBlock+`
		<form method="POST" action="/">
			<input type="hidden" name="csrf" value="`+html.EscapeString(csrfToken)+`">
			<input type="hidden" name="username" value="pilot" autocomplete="username">
			<label for="pw">Password (minimum 12 characters)</label>
			<input id="pw" name="password" type="password" minlength="12" maxlength="256" autocomplete="new-password" autofocus required>
			<label for="pw2">Confirm Password</label>
			<input id="pw2" name="confirm" type="password" minlength="12" maxlength="256" autocomplete="new-password" required>
			<button type="submit">Generate &amp; Save Hash</button>
		</form>
	`)
}

func main() {
	secretsDir := resolveSecretsDir()
	envPath := filepath.Join(secretsDir, "pw.env")
	absEnv, _ := filepath.Abs(envPath)

	csrfBytes := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, csrfBytes); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating CSRF token: %v\n", err)
		os.Exit(1)
	}
	csrfToken := hex.EncodeToString(csrfBytes)

	addr := os.Getenv("PASSGEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8089"
	}

	var (
		mu   sync.Mutex
		done bool
		srv  *http.Server
	)
	shutdownCh := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Frame-Options", "DENY")

		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(renderForm(csrfToken, absEnv, "")))
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		mu.Lock()
		if done {
			mu.Unlock()
			w.WriteHeader(http.StatusGone)
			_, _ = w.Write([]byte(renderPage("Completed", `<h1>COMPLETED</h1><p class="sub">Password hash was already saved. You may close this tab.</p>`)))
			return
		}
		mu.Unlock()

		_ = r.ParseForm()
		if subtle.ConstantTimeCompare([]byte(r.FormValue("csrf")), []byte(csrfToken)) != 1 {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(renderForm(csrfToken, absEnv, "Invalid session token. Reload and try again.")))
			return
		}

		pw := r.FormValue("password")
		confirm := r.FormValue("confirm")
		if pw != confirm {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(renderForm(csrfToken, absEnv, "Passwords do not match.")))
			return
		}

		hash, err := auth.HashPassword(pw)
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(renderForm(csrfToken, absEnv, err.Error())))
			return
		}

		mu.Lock()
		if done {
			mu.Unlock()
			w.WriteHeader(http.StatusGone)
			return
		}
		if err := upsertEnvVar(envPath, "ADMIN_PASSWORD_HASH", hash); err != nil {
			mu.Unlock()
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(renderForm(csrfToken, absEnv, "Failed to write pw.env: "+err.Error())))
			return
		}
		done = true
		mu.Unlock()

		_, _ = w.Write([]byte(renderPage("Saved — Res Imperii", `
			<h1>HASH SAVED</h1>
			<p class="ok">Argon2id password hash written to <code>`+html.EscapeString(absEnv)+`</code>.</p>
			<p class="sub">Save the password in Proton Pass if prompted. This one-time setup site is now shutting down — you can close this tab.</p>
		`)))

		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		go func() {
			time.Sleep(400 * time.Millisecond)
			close(shutdownCh)
		}()
	})

	srv = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		fmt.Printf("Res Imperii one-time password setup listening on http://%s\n", addr)
		fmt.Printf("Target secrets file: %s\n", absEnv)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	<-shutdownCh
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	fmt.Println("Password hash saved and one-time setup server stopped.")
}
