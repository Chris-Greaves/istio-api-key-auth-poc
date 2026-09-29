// Package keys generates and hashes API keys per ADR-0002 and persists
// their metadata.
package keys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

const (
	keyIDLength  = 8
	secretLength = 32
	idCharset    = "abcdefghijklmnopqrstuvwxyz0123456789"
)

// Generated is a freshly generated API key: the full plaintext key (shown to
// the operator exactly once), its plaintext Key ID (used for lookup and
// display), and the hash of its secret (what actually gets persisted).
type Generated struct {
	Plaintext  string
	KeyID      string
	SecretHash string
}

// Generate creates a new API key in the api_<Key ID>_<secret> format
// described in ADR-0002.
func Generate() (Generated, error) {
	keyID, err := randomString(keyIDLength)
	if err != nil {
		return Generated{}, fmt.Errorf("generating key id: %w", err)
	}

	secret, err := randomString(secretLength)
	if err != nil {
		return Generated{}, fmt.Errorf("generating secret: %w", err)
	}

	return Generated{
		Plaintext:  fmt.Sprintf("api_%s_%s", keyID, secret),
		KeyID:      keyID,
		SecretHash: HashSecret(secret),
	}, nil
}

// HashSecret hashes a key's secret portion for storage. The secret is
// high-entropy and randomly generated (unlike a user-chosen password), so a
// fast cryptographic hash is sufficient here — a slow, salted password hash
// defends against brute-forcing low-entropy input, which isn't the threat
// model for a 32-character random secret.
func HashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func randomString(n int) (string, error) {
	charsetSize := big.NewInt(int64(len(idCharset)))

	out := make([]byte, n)
	for i := range out {
		index, err := rand.Int(rand.Reader, charsetSize)
		if err != nil {
			return "", err
		}
		out[i] = idCharset[index.Int64()]
	}
	return string(out), nil
}
