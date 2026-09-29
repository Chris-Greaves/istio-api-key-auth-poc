package keys

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// maxGenerateAttempts bounds retries on the astronomically unlikely event of
// a Key ID collision, so a persistently broken generator or store fails loudly
// instead of looping forever.
const maxGenerateAttempts = 5

// CreateKey generates a new API key and persists its metadata, retrying
// generation if the Key ID happens to collide with an existing one.
func CreateKey(ctx context.Context, store *Store, owner string, expiresAt *time.Time) (string, Record, error) {
	for range maxGenerateAttempts {
		generated, err := Generate()
		if err != nil {
			return "", Record{}, fmt.Errorf("generating key: %w", err)
		}

		rec, err := store.Create(ctx, generated.KeyID, generated.SecretHash, owner, expiresAt)
		if err == nil {
			return generated.Plaintext, rec, nil
		}
		if !errors.Is(err, ErrKeyIDCollision) {
			return "", Record{}, err
		}
	}
	return "", Record{}, fmt.Errorf("failed to generate a unique key id after %d attempts", maxGenerateAttempts)
}
