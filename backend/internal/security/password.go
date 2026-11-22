package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 2
	argonKeyLen  uint32 = 32
	saltLength          = 16
)

// HashPassword produces an Argon2id PHC string using fixed application parameters.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

// VerifyPassword checks a password against a hash produced by HashPassword.
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	parameters := strings.Split(parts[3], ",")
	if len(parameters) != 3 {
		return false
	}
	memory, ok := parseParameter(parameters[0], "m")
	if !ok || memory != uint64(argonMemory) {
		return false
	}
	iterations, ok := parseParameter(parameters[1], "t")
	if !ok || iterations != uint64(argonTime) {
		return false
	}
	threads, ok := parseParameter(parameters[2], "p")
	if !ok || threads != uint64(argonThreads) {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != saltLength {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) != int(argonKeyLen) {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func parseParameter(value, name string) (uint64, bool) {
	key, raw, ok := strings.Cut(value, "=")
	if !ok || key != name {
		return 0, false
	}
	parsed, err := strconv.ParseUint(raw, 10, 32)
	return parsed, err == nil
}
