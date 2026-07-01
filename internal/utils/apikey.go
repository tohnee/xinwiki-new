package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
)

// apiKeyPrefixLen is the number of leading characters of the generated
// secret retained as the display Prefix (sk_ + 9 chars). The prefix lets
// operators identify a key in listings without exposing the secret.
const apiKeyPrefixLen = 12

// HashAPIKey returns the SHA-256 hex digest of an API key plaintext. API keys
// are credentials the server never needs to decrypt — it only compares them —
// so they are stored as hashes rather than AES-encrypted (unlike Tenant.APIKey,
// which is reversible because the legacy auth path compares plaintext).
func HashAPIKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// VerifyAPIKeyHash reports whether plaintext hashes to storedHash, using a
// constant-time comparison. An empty storedHash never verifies (fail-closed:
// a missing/revoked key with no hash must not match anything).
func VerifyAPIKeyHash(plaintext, storedHash string) bool {
	if storedHash == "" {
		return false
	}
	got := HashAPIKey(plaintext)
	return subtle.ConstantTimeCompare([]byte(got), []byte(storedHash)) == 1
}

// GenerateAPIKeySecret creates a new random API key secret ("sk_" + 32 hex
// chars) and its display prefix. The secret is returned in plaintext ONLY
// here; callers must hash it before persisting and return the plaintext to
// the user exactly once at creation time.
//
// Returns an error if the crypto/rand source fails, in which case callers
// MUST abort the operation rather than fall back to a weak RNG — a predictable
// API key is a critical security failure.
func GenerateAPIKeySecret() (secret, prefix string, err error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", fmt.Errorf("generate API key secret: crypto/rand failed: %w", err)
	}
	secret = "sk_" + hex.EncodeToString(b[:])
	prefix = secret
	if len(prefix) > apiKeyPrefixLen {
		prefix = prefix[:apiKeyPrefixLen]
	}
	return secret, prefix, nil
}

// GenerateAPIKeyID mints a new opaque key ID ("ak_" + 32 hex chars). The ID
// is the public handle persisted in api_keys.id and shown in listings; it
// carries no secret material, so unlike the plaintext secret it is safe to
// return on every read.
func GenerateAPIKeyID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate API key ID: crypto/rand failed: %w", err)
	}
	return "ak_" + hex.EncodeToString(b[:]), nil
}

// GenerateArtifactID mints a new opaque generated-artifact ID ("art_" + 32
// hex chars). Used as the primary key for generated_artifacts rows; carries
// no secret material and is safe to expose in listings and download links.
func GenerateArtifactID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate artifact ID: crypto/rand failed: %w", err)
	}
	return "art_" + hex.EncodeToString(b[:]), nil
}
