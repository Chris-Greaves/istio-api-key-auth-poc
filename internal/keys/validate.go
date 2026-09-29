package keys

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ValidationResult is the outcome of checking whether a plaintext API key is
// currently valid.
type ValidationResult struct {
	Valid  bool
	Reason string // set when Valid is false
	Owner  string // set when Valid is true
}

// Validate checks a plaintext API key (api_<Key ID>_<secret>) against the
// store: it must be well-formed, reference an existing key, have a secret
// matching the stored hash, and not be expired.
//
// A malformed key, an unknown Key ID, and a wrong secret all report the same
// generic "invalid key" reason — distinguishing them would let this endpoint
// be used to enumerate which Key IDs exist without knowing their secret.
// Expiry is reported specifically, since reaching that check already proves
// the caller holds the correct secret.
func Validate(ctx context.Context, store *Store, plaintext string) (ValidationResult, error) {
	const genericInvalidReason = "invalid key"

	keyID, secret, ok := ParseKey(plaintext)
	if !ok {
		return ValidationResult{Reason: genericInvalidReason}, nil
	}

	rec, secretHash, err := store.Get(ctx, keyID)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return ValidationResult{Reason: genericInvalidReason}, nil
		}
		return ValidationResult{}, fmt.Errorf("looking up key: %w", err)
	}

	if subtle.ConstantTimeCompare([]byte(HashSecret(secret)), []byte(secretHash)) != 1 {
		return ValidationResult{Reason: genericInvalidReason}, nil
	}

	if rec.ExpiresAt != nil && rec.ExpiresAt.Before(time.Now()) {
		return ValidationResult{Reason: "key has expired"}, nil
	}

	return ValidationResult{Valid: true, Owner: rec.Owner}, nil
}

// ParseKey splits a plaintext API key into its Key ID and secret, per the
// api_<Key ID>_<secret> format from ADR-0002. It reports false if plaintext
// doesn't match that shape.
func ParseKey(plaintext string) (keyID, secret string, ok bool) {
	const prefix = "api_"
	if !strings.HasPrefix(plaintext, prefix) {
		return "", "", false
	}

	rest := plaintext[len(prefix):]
	idx := strings.IndexByte(rest, '_')
	if idx <= 0 {
		return "", "", false
	}

	keyID, secret = rest[:idx], rest[idx+1:]
	if secret == "" {
		return "", "", false
	}
	return keyID, secret, true
}
