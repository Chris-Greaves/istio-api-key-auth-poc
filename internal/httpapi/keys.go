package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/cjgreaves97/istio-api-key-auth-poc/internal/keys"
)

// maxCreateKeyBodyBytes bounds the create-key request body so an
// unauthenticated caller (ADR-0003: no Web UI auth for the PoC) can't force
// unbounded JSON decoding on this endpoint.
const maxCreateKeyBodyBytes = 1 << 20 // 1 MiB

// maxOwnerLength bounds Owner, which the check endpoint (ADR-0001) injects
// verbatim as a response header that upstream services trust for
// attribution — not just display text, so it needs a sane size limit.
const maxOwnerLength = 256

// validOwner reports whether owner is non-empty, within maxOwnerLength, and
// free of control characters — required now that the check endpoint forwards
// it as a trusted response header rather than only ever displaying it.
func validOwner(owner string) bool {
	if owner == "" || len(owner) > maxOwnerLength {
		return false
	}
	for _, r := range owner {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

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
		var req createKeyRequest
		if !decodeJSONBody(w, r, maxCreateKeyBodyBytes, &req) {
			return
		}

		owner := strings.TrimSpace(req.Owner)
		if !validOwner(owner) {
			writeJSONError(w, http.StatusBadRequest, "owner is required, must not contain control characters, and must be at most 256 characters")
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

func revokeKeyHandler(store *keys.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keyID := r.PathValue("keyID")

		err := store.Revoke(r.Context(), keyID)
		if err != nil {
			if errors.Is(err, keys.ErrKeyNotFound) {
				writeJSONError(w, http.StatusNotFound, "key not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "failed to revoke key")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
