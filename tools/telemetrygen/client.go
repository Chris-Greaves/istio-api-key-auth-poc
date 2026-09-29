package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

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
	Key string `json:"key"`
}

// createKey calls the management API's create-key endpoint and returns the
// newly created key's plaintext value.
func (c *apiClient) createKey(ctx context.Context, owner string, expiresAt *time.Time) (string, error) {
	body, err := json.Marshal(createKeyRequest{Owner: owner, ExpiresAt: expiresAt})
	if err != nil {
		return "", fmt.Errorf("encoding create-key request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/keys", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("building create-key request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling create-key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("create-key returned %s: %s", resp.Status, bytes.TrimSpace(respBody))
	}

	var out createKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decoding create-key response: %w", err)
	}
	return out.Key, nil
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
