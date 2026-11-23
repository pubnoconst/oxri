package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFiles(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "db.env")
	content := "\uFEFF# comment written by a Windows editor (leading BOM)\n" +
		"CFG_URL=\"libsql://example.turso.io\"\n" +
		"export CFG_TOKEN='secret-token'\n" +
		"CFG_KEPT=from-file\n" +
		"not a pair\n"
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CFG_KEPT", "from-environment") // already set: must win
	for _, k := range []string{"CFG_URL", "CFG_TOKEN"} {
		k := k
		_ = os.Unsetenv(k)
		t.Cleanup(func() { _ = os.Unsetenv(k) })
	}

	if err := LoadEnvFiles(envPath, filepath.Join(dir, "missing.env")); err != nil {
		t.Fatalf("LoadEnvFiles: %v", err)
	}
	for k, want := range map[string]string{
		"CFG_URL":   "libsql://example.turso.io",
		"CFG_TOKEN": "secret-token",
		"CFG_KEPT":  "from-environment",
	} {
		if got := os.Getenv(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestLoadSecretsSearchOrder(t *testing.T) {
	// Layout: <tmp>/oxri-parent/db.env and <tmp>/db.env, run from <tmp>/oxri.
	root := t.TempDir()
	work := filepath.Join(root, "oxri")
	parent := filepath.Join(root, "oxri-parent")
	for _, d := range []string{work, parent} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p, s string) {
		if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(parent, "db.env"), "CFG_ORDER=canonical\n")
	write(filepath.Join(root, "db.env"), "CFG_ORDER=fallback\nCFG_ONLY_FALLBACK=yes\n")
	write(filepath.Join(parent, "pw.env"), "CFG_PW=hash\n")

	for _, k := range []string{"CFG_ORDER", "CFG_ONLY_FALLBACK", "CFG_PW"} {
		k := k
		_ = os.Unsetenv(k)
		t.Cleanup(func() { _ = os.Unsetenv(k) })
	}
	t.Chdir(work)

	if err := LoadSecrets("db.env"); err != nil {
		t.Fatalf("LoadSecrets: %v", err)
	}
	if got := os.Getenv("CFG_ORDER"); got != "canonical" {
		t.Errorf("../oxri-parent must win over ..: got %q", got)
	}
	if got := os.Getenv("CFG_ONLY_FALLBACK"); got != "yes" {
		t.Errorf("fallback directory not read: got %q", got)
	}
	if _, set := os.LookupEnv("CFG_PW"); set {
		t.Errorf("pw.env was not requested and must not be read")
	}
}
