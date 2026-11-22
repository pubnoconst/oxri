package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	// MinPasswordLength enforces a strong baseline password length.
	MinPasswordLength = 12

	// MaxPasswordLength prevents DoS via giant password inputs.
	MaxPasswordLength = 256

	argon2Time    uint32 = 3
	argon2Memory  uint32 = 64 * 1024 // 64 MiB
	argon2Threads uint8  = 2
	argon2KeyLen  uint32 = 32
	argon2SaltLen        = 16
)

// HashPassword hashes a plaintext password using Argon2id with a random 128-bit salt
// and returns a standard PHC string: $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>.
func HashPassword(password string) (string, error) {
	if len(password) < MinPasswordLength {
		return "", fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	if len(password) > MaxPasswordLength {
		return "", fmt.Errorf("password must be at most %d characters", MaxPasswordLength)
	}

	salt := make([]byte, argon2SaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argon2Memory, argon2Time, argon2Threads, b64Salt, b64Hash), nil
}

// ValidatePasswordHash checks whether encodedHash is a structurally valid Argon2id PHC string.
func ValidatePasswordHash(encodedHash string) error {
	_, _, _, _, _, err := parseArgon2idHash(encodedHash)
	return err
}

// VerifyPassword checks plaintext password against an Argon2id PHC hash in constant time.
func VerifyPassword(encodedHash, password string) bool {
	if len(password) == 0 || len(password) > MaxPasswordLength {
		return false
	}
	memory, iterations, parallelism, salt, expectedHash, err := parseArgon2idHash(encodedHash)
	if err != nil {
		return false
	}
	actualHash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expectedHash)))
	return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1
}

func parseArgon2idHash(encoded string) (memory, iterations uint32, parallelism uint8, salt, hash []byte, err error) {
	encoded = strings.TrimSpace(encoded)
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return 0, 0, 0, nil, nil, errors.New("invalid argon2id hash format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return 0, 0, 0, nil, nil, errors.New("unsupported argon2id version")
	}

	var mem64, iter64, par64 uint64
	for _, kv := range strings.Split(parts[3], ",") {
		pair := strings.SplitN(kv, "=", 2)
		if len(pair) != 2 {
			return 0, 0, 0, nil, nil, errors.New("invalid argon2id parameter")
		}
		v, parseErr := strconv.ParseUint(pair[1], 10, 32)
		if parseErr != nil || v == 0 {
			return 0, 0, 0, nil, nil, errors.New("invalid argon2id parameter value")
		}
		switch pair[0] {
		case "m":
			mem64 = v
		case "t":
			iter64 = v
		case "p":
			par64 = v
		default:
			return 0, 0, 0, nil, nil, errors.New("unknown argon2id parameter")
		}
	}
	if mem64 == 0 || iter64 == 0 || par64 == 0 || par64 > 255 {
		return 0, 0, 0, nil, nil, errors.New("incomplete argon2id parameters")
	}

	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return 0, 0, 0, nil, nil, errors.New("invalid argon2id salt")
	}
	hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(hash) < 16 {
		return 0, 0, 0, nil, nil, errors.New("invalid argon2id hash bytes")
	}

	return uint32(mem64), uint32(iter64), uint8(par64), salt, hash, nil
}
