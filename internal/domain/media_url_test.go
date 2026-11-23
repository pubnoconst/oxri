package domain_test

import (
	"testing"

	"github.com/oxri/oxri-go/internal/domain"
)

func TestParseMediaURL(t *testing.T) {
	validCases := []struct {
		input    string
		wantHost string
	}{
		{"https://example.com/image.png", "example.com"},
		{"https://cdn.oxri.org/assets/photo.webp?v=1", "cdn.oxri.org"},
		{"https://images.unsplash.com/photo-12345", "images.unsplash.com"},
		{"https://8.8.8.8/image.png", "8.8.8.8"}, // Public IP is permitted
	}

	for _, tc := range validCases {
		m, err := domain.ParseMediaURL(tc.input)
		if err != nil {
			t.Fatalf("ParseMediaURL(%q) unexpected error: %v", tc.input, err)
		}
		if m.Host() != tc.wantHost {
			t.Errorf("ParseMediaURL(%q).Host() = %q, want %q", tc.input, m.Host(), tc.wantHost)
		}
		if m.String() != tc.input {
			t.Errorf("ParseMediaURL(%q).String() = %q, want %q", tc.input, m.String(), tc.input)
		}
		if m.IsZero() {
			t.Errorf("ParseMediaURL(%q).IsZero() returned true", tc.input)
		}
	}

	invalidCases := []string{
		"",                                         // empty
		"   ",                                      // whitespace
		"http://example.com/insecure.png",          // insecure scheme
		"ftp://example.com/file.png",               // unsupported scheme
		"javascript:alert(1)",                      // XSS uri
		"file:///etc/passwd",                       // local file
		"https://localhost/image.png",              // localhost
		"https://sub.localhost/image.png",          // sub-localhost
		"https://device.local/image.png",           // .local domain
		"https://127.0.0.1/secret",                 // loopback IPv4
		"https://[::1]/secret",                     // loopback IPv6
		"https://10.0.0.1/admin",                   // RFC 1918 private IPv4
		"https://172.16.5.1/internal",              // RFC 1918 private IPv4
		"https://192.168.1.1/router",               // RFC 1918 private IPv4
		"https://169.254.169.254/latest/meta-data", // Cloud metadata link-local SSRF
	}

	for _, input := range invalidCases {
		_, err := domain.ParseMediaURL(input)
		if err == nil {
			t.Errorf("ParseMediaURL(%q) expected error, got nil", input)
		}
	}
}
