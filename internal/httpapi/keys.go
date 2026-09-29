package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/cjgreaves97/istio-api-key-auth-poc/internal/keys"
)

// maxCreateKeyBodyBytes bounds the create-key request body so an
// unauthenticated caller (ADR-0003: no Web UI auth for the PoC) can't force
// unbounded JSON decoding on this endpoint.
const maxCreateKeyBodyBytes = 1 << 20 // 1 MiB

type createKeyRequest struct {
	Owner     string     `json:"owner"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// keyResponse is a key's metadata as returned to API callers — it never
// includes the secret or its hash.
type keyResponse struct {
	KeyID     string     `json:"key_id"`
	Owner     string     `json:"owner"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type createKeyResponse struct {
	Key string `json:"key"`
	keyResponse
}

func toKeyResponse(rec keys.Record) keyResponse {
	return keyResponse{
		KeyID:     rec.KeyID,
		Owner:     rec.Owner,
		CreatedAt: rec.CreatedAt,
		ExpiresAt: rec.ExpiresAt,
	}
}

func createKeyHandler(store *keys.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxCreateKeyBodyBytes)

		var req createKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		owner := strings.TrimSpace(req.Owner)
		if owner == "" {
			writeJSONError(w, http.StatusBadRequest, "owner is required")
			return
		}

		plaintext, rec, err := keys.CreateKey(r.Context(), store, owner, req.ExpiresAt)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to create key")
			return
		}

		writeJSON(w, http.StatusCreated, createKeyResponse{
			Key:         plaintext,
			keyResponse: toKeyResponse(rec),
		})
	}
}

func listKeysHandler(store *keys.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		records, err := store.List(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to list keys")
			return
		}

		resp := make([]keyResponse, 0, len(records))
		for _, rec := range records {
			resp = append(resp, toKeyResponse(rec))
		}

		writeJSON(w, http.StatusOK, resp)
	}
}
