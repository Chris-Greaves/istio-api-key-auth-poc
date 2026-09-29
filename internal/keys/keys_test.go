package keys_test

import (
	"regexp"
	"testing"

	"github.com/cjgreaves97/istio-api-key-auth-poc/internal/keys"
)

var fullKeyPattern = regexp.MustCompile(`^api_[a-z0-9]{8}_[a-z0-9]{32}$`)

func TestGenerate_ProducesKeyInExpectedFormat(t *testing.T) {
	generated, err := keys.Generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fullKeyPattern.MatchString(generated.Plaintext) {
		t.Fatalf("expected plaintext key to match %q, got %q", fullKeyPattern.String(), generated.Plaintext)
	}

	if len(generated.KeyID) != 8 {
		t.Fatalf("expected key id to be 8 characters, got %q", generated.KeyID)
	}
}

func TestGenerate_PlaintextEmbedsTheKeyID(t *testing.T) {
	generated, err := keys.Generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPrefix := "api_" + generated.KeyID + "_"
	if len(generated.Plaintext) <= len(expectedPrefix) || generated.Plaintext[:len(expectedPrefix)] != expectedPrefix {
		t.Fatalf("expected plaintext %q to start with %q", generated.Plaintext, expectedPrefix)
	}
}

func TestGenerate_SecretHashDoesNotContainTheSecretOrPlaintext(t *testing.T) {
	generated, err := keys.Generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if generated.SecretHash == "" {
		t.Fatal("expected a non-empty secret hash")
	}
	if generated.SecretHash == generated.Plaintext {
		t.Fatal("expected secret hash to differ from the plaintext key")
	}
}

func TestGenerate_ProducesUniqueKeysAcrossCalls(t *testing.T) {
	first, err := keys.Generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := keys.Generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first.KeyID == second.KeyID {
		t.Fatalf("expected distinct key ids, got %q twice", first.KeyID)
	}
	if first.Plaintext == second.Plaintext {
		t.Fatalf("expected distinct plaintext keys, got %q twice", first.Plaintext)
	}
}

func TestHashSecret_IsDeterministic(t *testing.T) {
	first := keys.HashSecret("some-secret")
	second := keys.HashSecret("some-secret")
	if first != second {
		t.Fatal("expected hashing the same secret twice to produce the same hash")
	}
}

func TestHashSecret_DiffersForDifferentSecrets(t *testing.T) {
	if keys.HashSecret("secret-one") == keys.HashSecret("secret-two") {
		t.Fatal("expected different secrets to hash to different values")
	}
}
