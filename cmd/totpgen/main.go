package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oxri/oxri-go/internal/auth"
	"github.com/skip2/go-qrcode"
)

func resolveSecretsDir() string {
	parentDir := filepath.Join("..", "oxri-parent")
	if info, err := os.Stat(parentDir); err == nil && info.IsDir() {
		return parentDir
	}
	return ".."
}

func readExistingSecret(envPath string) (string, string) {
	data, err := os.ReadFile(envPath)
	if err != nil {
		return "", ""
	}
	content := string(data)
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "ADMIN_TOTP_SECRET=") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "ADMIN_TOTP_SECRET="))
			val = strings.Trim(val, `"'`)
			return auth.NormalizeTOTPSecret(val), content
		}
	}
	return "", content
}

func upsertEnvVar(envPath, existingContent, key, value string) error {
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
		// Avoid double trailing blank lines
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

func main() {
	force := flag.Bool("force", false, "Overwrite any existing ADMIN_TOTP_SECRET in db.env with a new seed")
	issuer := flag.String("issuer", "Res Imperii", "Authenticator issuer label")
	account := flag.String("account", "pilot", "Authenticator account label")
	flag.Parse()

	secretsDir := resolveSecretsDir()
	envPath := filepath.Join(secretsDir, "db.env")
	qrPath := filepath.Join(secretsDir, "totp-qr.png")

	secret, existingContent := readExistingSecret(envPath)
	created := false
	if secret == "" || *force {
		var err error
		secret, err = auth.GenerateTOTPSecret()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating TOTP secret: %v\n", err)
			os.Exit(1)
		}
		if err := upsertEnvVar(envPath, existingContent, "ADMIN_TOTP_SECRET", secret); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", envPath, err)
			os.Exit(1)
		}
		created = true
	}

	uri := auth.BuildTOTPURI(secret, *issuer, *account)
	pngBytes, err := qrcode.Encode(uri, qrcode.Medium, 320)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding TOTP QR code: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(qrPath, pngBytes, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", qrPath, err)
		os.Exit(1)
	}

	absEnv, _ := filepath.Abs(envPath)
	absQR, _ := filepath.Abs(qrPath)

	if created {
		fmt.Printf("Generated new ADMIN_TOTP_SECRET and saved it to:\n  %s\n", absEnv)
	} else {
		fmt.Printf("Kept existing ADMIN_TOTP_SECRET from:\n  %s\n", absEnv)
	}
	fmt.Printf("Saved scannable TOTP QR code to:\n  %s\n", absQR)
	fmt.Println("Scan the QR code into Proton Pass (or your authenticator app), then delete totp-qr.png when done.")
}
