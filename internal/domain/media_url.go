package domain

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

var (
	ErrInvalidMediaURL = errors.New("domain: invalid media URL")
	ErrInsecureScheme  = errors.New("domain: media URL must use https scheme")
	ErrEmptyHost       = errors.New("domain: media URL must specify a non-empty host")
	ErrPrivateHost     = errors.New("domain: media URL host must not be localhost or a private/internal IP")
)

// MediaURL is an opaque domain type representing a strictly validated,
// HTTPS-only remote media asset URL. It protects against SSRF by disallowing
// internal IP ranges, loopback, and localhost.
type MediaURL struct {
	raw  string
	host string
}

// ParseMediaURL parses and validates a raw URL string into a MediaURL.
func ParseMediaURL(raw string) (MediaURL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return MediaURL{}, fmt.Errorf("%w: URL is empty", ErrInvalidMediaURL)
	}

	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return MediaURL{}, fmt.Errorf("%w: %v", ErrInvalidMediaURL, err)
	}

	if !strings.EqualFold(parsed.Scheme, "https") {
		return MediaURL{}, ErrInsecureScheme
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return MediaURL{}, ErrEmptyHost
	}

	// Disallow localhost and common local hostnames
	lowerHost := strings.ToLower(hostname)
	if lowerHost == "localhost" || strings.HasSuffix(lowerHost, ".localhost") || strings.HasSuffix(lowerHost, ".local") {
		return MediaURL{}, ErrPrivateHost
	}

	// If host is an IP address, reject private/loopback/link-local ranges
	if ip := net.ParseIP(hostname); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return MediaURL{}, ErrPrivateHost
		}
	}

	return MediaURL{
		raw:  parsed.String(),
		host: parsed.Host,
	}, nil
}

// String returns the canonical validated URL string.
func (m MediaURL) String() string {
	return m.raw
}

// Host returns the host component of the media URL.
func (m MediaURL) Host() string {
	return m.host
}

// IsZero reports whether the MediaURL is the zero value.
func (m MediaURL) IsZero() bool {
	return m.raw == ""
}
