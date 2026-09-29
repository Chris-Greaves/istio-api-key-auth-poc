package httpapi

import (
	"net/http"
	"strings"

	"github.com/cjgreaves97/istio-api-key-auth-poc/internal/keys"
)

// maxValidateKeyBodyBytes bounds the validate-key request body so an
// unauthenticated caller (ADR-0003: no Web UI auth for the PoC) can't force
// unbounded JSON decoding on this endpoint.
const maxValidateKeyBodyBytes = 1 << 10 // 1 KiB — a single API key is tiny

type validateKeyRequest struct {
	Key string `json:"key"`
}

type validateKeyResponse struct {
	Valid  bool   `json:"valid"`
	Reason string `json:"reason,omitempty"`
	Owner  string `json:"owner,omitempty"`
}

// validateKeyHandler lets an operator check whether a plaintext key they
// hold is currently valid — a UI convenience distinct from the Istio-facing
// ext_authz check endpoint (ADR-0001), which authenticates live mesh traffic
// rather than answering an interactive query. It always responds 200; the
// result (valid or not, and why) is carried in the body.
func validateKeyHandler(store *keys.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req validateKeyRequest
		if !decodeJSONBody(w, r, maxValidateKeyBodyBytes, &req) {
			return
		}

		key := strings.TrimSpace(req.Key)
		if key == "" {
			writeJSONError(w, http.StatusBadRequest, "key is required")
			return
		}

		result, err := keys.Validate(r.Context(), store, key)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to validate key")
			return
		}

		writeJSON(w, http.StatusOK, validateKeyResponse{
			Valid:  result.Valid,
			Reason: result.Reason,
			Owner:  result.Owner,
		})
	}
}
