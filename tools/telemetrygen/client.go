package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrKeyNotFound indicates the management API reported no active key with
// the given Key ID — e.g. a revoke call racing another revoke of the same
// key.
var ErrKeyNotFound = errors.New("key not found")

// apiClient talks to a single running instance of the service: the
// management API for bootstrapping keys, and the Check Service's
// /authz/check endpoint for the check stream.
type apiClient struct {
	baseURL string
	http    *http.Client
}

func newAPIClient(baseURL string) *apiClient {
	return &apiClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

type createKeyRequest struct {
	Owner     string     `json:"owner"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type createKeyResponse struct {
	Key   string `json:"key"`
	KeyID string `json:"key_id"`
}

// createKey calls the management API's create-key endpoint and returns the
// newly created key's plaintext value and Key ID.
func (c *apiClient) createKey(ctx context.Context, owner string, expiresAt *time.Time) (PoolKey, error) {
	body, err := json.Marshal(createKeyRequest{Owner: owner, ExpiresAt: expiresAt})
	if err != nil {
		return PoolKey{}, fmt.Errorf("encoding create-key request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/keys", bytes.NewReader(body))
	if err != nil {
		return PoolKey{}, fmt.Errorf("building create-key request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return PoolKey{}, fmt.Errorf("calling create-key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return PoolKey{}, fmt.Errorf("create-key returned %s: %s", resp.Status, bytes.TrimSpace(respBody))
	}

	var out createKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return PoolKey{}, fmt.Errorf("decoding create-key response: %w", err)
	}
	return PoolKey{KeyID: out.KeyID, Plaintext: out.Key}, nil
}

type listKeysEntry struct {
	KeyID string `json:"key_id"`
}

// listKeys calls the management API's list-keys endpoint and returns the Key
// IDs of every currently active key.
func (c *apiClient) listKeys(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/keys", nil)
	if err != nil {
		return nil, fmt.Errorf("building list-keys request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling list-keys: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("list-keys returned %s: %s", resp.Status, bytes.TrimSpace(respBody))
	}

	var out []listKeysEntry
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decoding list-keys response: %w", err)
	}

	ids := make([]string, len(out))
	for i, entry := range out {
		ids[i] = entry.KeyID
	}
	return ids, nil
}

// revokeKey calls the management API's revoke-key endpoint for keyID. It
// returns ErrKeyNotFound if the service reports no active key with that ID.
func (c *apiClient) revokeKey(ctx context.Context, keyID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/api/keys/"+url.PathEscape(keyID), nil)
	if err != nil {
		return fmt.Errorf("building revoke-key request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling revoke-key: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return ErrKeyNotFound
	default:
		return fmt.Errorf("revoke-key returned %s", resp.Status)
	}
}

// scrapeMetrics fetches the service's /metrics endpoint and returns its body
// as Prometheus exposition-format text, for the final summary's metrics
// diff.
func (c *apiClient) scrapeMetrics(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/metrics", nil)
	if err != nil {
		return "", fmt.Errorf("building metrics request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling metrics: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading metrics response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("metrics returned %s: %s", resp.Status, bytes.TrimSpace(body))
	}
	return string(body), nil
}

// check sends a single request to the Check Service's /authz/check endpoint,
// presenting key via X-API-Key (or omitting the header entirely when key is
// empty), and returns the response status code.
func (c *apiClient) check(ctx context.Context, key string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/authz/check", nil)
	if err != nil {
		return 0, fmt.Errorf("building check request: %w", err)
	}
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("calling check: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	return resp.StatusCode, nil
}
