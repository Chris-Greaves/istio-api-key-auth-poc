package keys_test

import (
	"testing"

	"github.com/cjgreaves97/istio-api-key-auth-poc/internal/keys"
)

func TestParseKey_AcceptsAWellFormedKey(t *testing.T) {
	keyID, secret, ok := keys.ParseKey("api_abcd1234_" + "s3cr3t")
	if !ok {
		t.Fatal("expected a well-formed key to parse successfully")
	}
	if keyID != "abcd1234" {
		t.Fatalf("expected key id %q, got %q", "abcd1234", keyID)
	}
	if secret != "s3cr3t" {
		t.Fatalf("expected secret %q, got %q", "s3cr3t", secret)
	}
}

func TestParseKey_RejectsMalformedInput(t *testing.T) {
	cases := []string{
		"",
		"not-a-key",
		"api_",
		"api_onlyonepart",
		"api__missing-key-id",
		"api_abcd1234_",
		"wrong-prefix_abcd1234_secret",
	}

	for _, c := range cases {
		if _, _, ok := keys.ParseKey(c); ok {
			t.Errorf("expected %q to be rejected as malformed", c)
		}
	}
}
