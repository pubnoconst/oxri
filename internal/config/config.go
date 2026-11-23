// Package config loads process configuration from secret env files.
//
// Secrets live outside the repository (Principia #7): in ../oxri-parent/ (the
// canonical location) or, as a fallback, in the directory above the checkout.
// Variables already present in the process environment always win, so a
// platform (Render) or a shell can override any file.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// secretDirs are searched in order; the first file to set a variable wins.
var secretDirs = []string{
	filepath.Join("..", "oxri-parent"),
	"..",
}

// LoadSecrets loads the named env files (e.g. "db.env", "pw.env"), plus a
// generic ".env", from each secrets directory in turn. Each process names only
// the files it needs (the backup tool, for example, never reads pw.env).
// Missing files are ignored.
func LoadSecrets(names ...string) error {
	files := make([]string, 0, len(secretDirs)*(len(names)+1))
	for _, dir := range secretDirs {
		for _, name := range append(append([]string(nil), names...), ".env") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	return LoadEnvFiles(files...)
}

// LoadEnvFiles reads KEY=value lines from each path into the process
// environment without overriding variables that are already set. It accepts
// comments, blank lines, an optional "export " prefix, matching outer quotes,
// and a leading UTF-8 byte-order mark (written by some Windows editors).
// Missing files are ignored.
func LoadEnvFiles(paths ...string) error {
	for _, p := range paths {
		if err := loadEnvFile(p); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
	}
	return nil
}

func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\uFEFF"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		if key == "" {
			continue
		}
		if len(val) >= 2 && (val[0] == '"' && val[len(val)-1] == '"' || val[0] == '\'' && val[len(val)-1] == '\'') {
			val = val[1 : len(val)-1]
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	return scanner.Err()
}
