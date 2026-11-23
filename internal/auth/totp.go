package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

const (
	// TOTPPeriodSeconds is the standard RFC 6238 time-step duration (30s).
	TOTPPeriodSeconds = 30

	// TOTPDigits is the standard 6-digit authenticator code length.
	TOTPDigits = 6

	// totpMaxFailures caps failed TOTP attempts within totpLockoutWindow.
	totpMaxFailures   = 5
	totpLockoutWindow = 5 * time.Minute
)

var (
	ErrTOTPNotConfigured = errors.New("auth: TOTP is not configured")
	ErrTOTPInvalid       = errors.New("auth: invalid or expired authentication code")
	ErrTOTPReplayed      = errors.New("auth: authentication code already used")
	ErrTOTPLocked        = errors.New("auth: too many failed authentication attempts; please wait")
)

var totpBase32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// TOTPReplayStore persists the highest accepted TOTP time step across process restarts.
type TOTPReplayStore interface {
	LoadLastTOTPStep(ctx context.Context) (int64, error)
	SaveLastTOTPStep(ctx context.Context, step int64) error
}

// GenerateTOTPSecret creates a cryptographically random 160-bit Base32 TOTP seed.
func GenerateTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", err
	}
	return totpBase32.EncodeToString(raw), nil
}

// NormalizeTOTPSecret trims whitespace/hyphens and uppercases a Base32 TOTP seed.
func NormalizeTOTPSecret(secret string) string {
	s := strings.TrimSpace(secret)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.TrimRight(s, "=")
	return strings.ToUpper(s)
}

func decodeTOTPSecret(secret string) ([]byte, error) {
	norm := NormalizeTOTPSecret(secret)
	if len(norm) < 16 {
		return nil, errors.New("TOTP secret is too short")
	}
	return totpBase32.DecodeString(norm)
}

// BuildTOTPURI constructs a standard otpauth:// URI compatible with Proton Pass
// and all RFC 6238 authenticator apps.
func BuildTOTPURI(secret, issuer, account string) string {
	norm := NormalizeTOTPSecret(secret)
	if issuer == "" {
		issuer = "Res Imperii"
	}
	if account == "" {
		account = "pilot"
	}
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{}
	q.Set("secret", norm)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprintf("%d", TOTPDigits))
	q.Set("period", fmt.Sprintf("%d", TOTPPeriodSeconds))
	return fmt.Sprintf("otpauth://totp/%s?%s", label, q.Encode())
}

// GenerateTOTPCode computes the RFC 6238 6-digit TOTP code for the given secret and time.
func GenerateTOTPCode(secret string, at time.Time) (string, error) {
	key, err := decodeTOTPSecret(secret)
	if err != nil {
		return "", err
	}
	step := at.UTC().Unix() / TOTPPeriodSeconds
	return hotpCode(key, uint64(step)), nil
}

func hotpCode(key []byte, counter uint64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(buf[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	codeInt := (uint32(sum[offset]&0x7f) << 24) |
		(uint32(sum[offset+1]) << 16) |
		(uint32(sum[offset+2]) << 8) |
		uint32(sum[offset+3])

	return fmt.Sprintf("%06d", codeInt%1000000)
}

// VerifyTOTP validates a 6-digit TOTP code against the secret with ±1 time-step skew tolerance.
// On success, it returns the matched time step so callers can enforce single-use replay protection.
func VerifyTOTP(secret, code string, now time.Time) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != TOTPDigits {
		return 0, false
	}
	for i := 0; i < TOTPDigits; i++ {
		if code[i] < '0' || code[i] > '9' {
			return 0, false
		}
	}

	key, err := decodeTOTPSecret(secret)
	if err != nil {
		return 0, false
	}

	currentStep := now.UTC().Unix() / TOTPPeriodSeconds
	for _, delta := range []int64{0, -1, 1} {
		step := currentStep + delta
		if step < 0 {
			continue
		}
		expected := hotpCode(key, uint64(step))
		if subtle.ConstantTimeCompare([]byte(code), []byte(expected)) == 1 {
			return step, true
		}
	}
	return 0, false
}
