package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/keys"
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
		ctx, span := tracer.Start(r.Context(), "management_api.create_key")
		defer span.End()

		var req createKeyRequest
		if !decodeJSONBody(w, r, maxCreateKeyBodyBytes, &req) {
			recordManagementRequest(ctx, "create_key", "invalid_request", "")
			return
		}

		owner := strings.TrimSpace(req.Owner)
		if !validOwner(owner) {
			failManagement(ctx, w, http.StatusBadRequest, "owner is required, must not contain control characters, and must be at most 256 characters", "create_key", "invalid_request")
			return
		}

		plaintext, rec, err := keys.CreateKey(ctx, store, owner, req.ExpiresAt)
		if err != nil {
			failManagement(ctx, w, http.StatusInternalServerError, "failed to create key", "create_key", "error")
			return
		}

		recordManagementRequest(ctx, "create_key", "success", rec.KeyID)
		writeJSON(w, http.StatusCreated, createKeyResponse{
			Key:         plaintext,
			keyResponse: toKeyResponse(rec),
		})
	}
}

func listKeysHandler(store *keys.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "management_api.list_keys")
		defer span.End()

		records, err := store.List(ctx)
		if err != nil {
			failManagement(ctx, w, http.StatusInternalServerError, "failed to list keys", "list_keys", "error")
			return
		}

		resp := make([]keyResponse, 0, len(records))
		for _, rec := range records {
			resp = append(resp, toKeyResponse(rec))
		}

		recordManagementRequest(ctx, "list_keys", "success", "")
		writeJSON(w, http.StatusOK, resp)
	}
}

func revokeKeyHandler(store *keys.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "management_api.revoke_key")
		defer span.End()

		// requestedKeyID is unverified caller input (the store hasn't
		// confirmed it names a real key) — never pass it to failManagement's
		// underlying telemetry as a key_id label, or an unauthenticated
		// caller (ADR-0003) could mint arbitrary label values. Once
		// store.Revoke succeeds it's a confirmed real Key ID, safe to record.
		requestedKeyID := r.PathValue("keyID")

		err := store.Revoke(ctx, requestedKeyID)
		if err != nil {
			if errors.Is(err, keys.ErrKeyNotFound) {
				failManagement(ctx, w, http.StatusNotFound, "key not found", "revoke_key", "not_found")
				return
			}
			failManagement(ctx, w, http.StatusInternalServerError, "failed to revoke key", "revoke_key", "error")
			return
		}

		recordManagementRequest(ctx, "revoke_key", "success", requestedKeyID)
		w.WriteHeader(http.StatusNoContent)
	}
}
