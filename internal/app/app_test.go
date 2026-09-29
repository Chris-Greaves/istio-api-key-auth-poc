package app_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/cjgreaves97/istio-api-key-auth-poc/internal/app"
	"github.com/cjgreaves97/istio-api-key-auth-poc/internal/config"
)

func startPostgres(t *testing.T) string {
	t.Helper()

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("apikeys"),
		postgres.WithUsername("apikeys"),
		postgres.WithPassword("apikeys"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("starting postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(context.Background()); err != nil {
			t.Logf("terminating postgres container: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("getting postgres connection string: %v", err)
	}

	return connStr
}

func TestApp_HealthyAndSchemaMigratedAfterStartup(t *testing.T) {
	connStr := startPostgres(t)
	ctx := context.Background()

	application, err := app.New(ctx, config.Config{DatabaseURL: connStr})
	if err != nil {
		t.Fatalf("starting application: %v", err)
	}
	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Logf("closing application: %v", err)
		}
	})

	server := httptest.NewServer(application.Handler())
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("calling /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected /healthz to return %d, got %d", http.StatusOK, resp.StatusCode)
	}

	verifyDB, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("opening verification connection: %v", err)
	}
	defer verifyDB.Close()

	var exists bool
	err = verifyDB.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'keys')`,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("checking for keys table: %v", err)
	}
	if !exists {
		t.Fatal("expected keys table to exist after migrations ran")
	}
}

func TestApp_HealthzReportsUnhealthyOnceDatabaseIsClosed(t *testing.T) {
	connStr := startPostgres(t)
	ctx := context.Background()

	application, err := app.New(ctx, config.Config{DatabaseURL: connStr})
	if err != nil {
		t.Fatalf("starting application: %v", err)
	}

	server := httptest.NewServer(application.Handler())
	t.Cleanup(server.Close)

	if err := application.Close(); err != nil {
		t.Fatalf("closing application: %v", err)
	}

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("calling /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected /healthz to return %d once the database connection is closed, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}
}

var fullKeyPattern = regexp.MustCompile(`^api_[a-z0-9]{8}_[a-z0-9]{32}$`)

func startServer(t *testing.T) *httptest.Server {
	t.Helper()

	connStr := startPostgres(t)
	application, err := app.New(context.Background(), config.Config{DatabaseURL: connStr})
	if err != nil {
		t.Fatalf("starting application: %v", err)
	}
	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Logf("closing application: %v", err)
		}
	})

	server := httptest.NewServer(application.Handler())
	t.Cleanup(server.Close)
	return server
}

type keyResponse struct {
	Key       string  `json:"key"`
	KeyID     string  `json:"key_id"`
	Owner     string  `json:"owner"`
	CreatedAt string  `json:"created_at"`
	ExpiresAt *string `json:"expires_at"`
}

func TestApp_CreateKeyReturnsFullKeyOnceAndTheKeyThenAppearsInTheList(t *testing.T) {
	server := startServer(t)

	createResp, err := http.Post(
		server.URL+"/api/keys",
		"application/json",
		strings.NewReader(`{"owner":"test-owner"}`),
	)
	if err != nil {
		t.Fatalf("calling create-key: %v", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected create-key to return %d, got %d", http.StatusCreated, createResp.StatusCode)
	}

	var created keyResponse
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decoding create-key response: %v", err)
	}

	if !fullKeyPattern.MatchString(created.Key) {
		t.Fatalf("expected returned key to match %q, got %q", fullKeyPattern.String(), created.Key)
	}
	if created.Owner != "test-owner" {
		t.Fatalf("expected owner %q, got %q", "test-owner", created.Owner)
	}
	if created.KeyID == "" {
		t.Fatal("expected a non-empty key id")
	}

	listResp, err := http.Get(server.URL + "/api/keys")
	if err != nil {
		t.Fatalf("calling list-keys: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("expected list-keys to return %d, got %d", http.StatusOK, listResp.StatusCode)
	}

	body, err := io.ReadAll(listResp.Body)
	if err != nil {
		t.Fatalf("reading list-keys response: %v", err)
	}

	var listed []keyResponse
	if err := json.Unmarshal(body, &listed); err != nil {
		t.Fatalf("decoding list-keys response: %v", err)
	}

	found := false
	for _, k := range listed {
		if k.KeyID == created.KeyID {
			found = true
			if k.Owner != "test-owner" {
				t.Fatalf("expected listed owner %q, got %q", "test-owner", k.Owner)
			}
		}
	}
	if !found {
		t.Fatalf("expected created key %q to appear in the list, got %+v", created.KeyID, listed)
	}

	var rawListed []map[string]any
	if err := json.Unmarshal(body, &rawListed); err != nil {
		t.Fatalf("decoding list-keys response as raw JSON: %v", err)
	}
	rawFound := false
	for _, k := range rawListed {
		if k["key_id"] != created.KeyID {
			continue
		}
		rawFound = true
		expiresAt, present := k["expires_at"]
		if !present {
			t.Fatal("expected expires_at to be present (as null) for a key with no expiry, but the field was omitted")
		}
		if expiresAt != nil {
			t.Fatalf("expected expires_at to be null for a key with no expiry, got %v", expiresAt)
		}
	}
	if !rawFound {
		t.Fatalf("expected created key %q to appear in the raw list response", created.KeyID)
	}

	if strings.Contains(string(body), created.Key) {
		t.Fatal("expected list-keys response to never contain the full plaintext key")
	}
	if bytes.Contains(body, []byte("secret")) {
		t.Fatal("expected list-keys response to never mention the secret or its hash")
	}
}

func TestApp_CreateKeyWithoutOwnerIsRejected(t *testing.T) {
	server := startServer(t)

	resp, err := http.Post(
		server.URL+"/api/keys",
		"application/json",
		strings.NewReader(`{}`),
	)
	if err != nil {
		t.Fatalf("calling create-key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected create-key without an owner to return %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestApp_CreateKeyWithWhitespaceOnlyOwnerIsRejected(t *testing.T) {
	server := startServer(t)

	resp, err := http.Post(
		server.URL+"/api/keys",
		"application/json",
		strings.NewReader(`{"owner":"   "}`),
	)
	if err != nil {
		t.Fatalf("calling create-key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected create-key with a whitespace-only owner to return %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func createTestKey(t *testing.T, server *httptest.Server, requestBody string) keyResponse {
	t.Helper()

	resp, err := http.Post(server.URL+"/api/keys", "application/json", strings.NewReader(requestBody))
	if err != nil {
		t.Fatalf("calling create-key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected create-key to return %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var created keyResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decoding create-key response: %v", err)
	}
	return created
}

type validateKeyResponse struct {
	Valid  bool   `json:"valid"`
	Reason string `json:"reason"`
	Owner  string `json:"owner"`
}

func validateTestKey(t *testing.T, server *httptest.Server, key string) (int, validateKeyResponse) {
	t.Helper()

	body, err := json.Marshal(map[string]string{"key": key})
	if err != nil {
		t.Fatalf("marshaling validate-key request: %v", err)
	}

	resp, err := http.Post(server.URL+"/api/keys/validate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("calling validate-key: %v", err)
	}
	defer resp.Body.Close()

	var result validateKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decoding validate-key response: %v", err)
	}
	return resp.StatusCode, result
}

func TestApp_ValidateKeyAcceptsAValidActiveKey(t *testing.T) {
	server := startServer(t)
	created := createTestKey(t, server, `{"owner":"validate-owner"}`)

	status, result := validateTestKey(t, server, created.Key)

	if status != http.StatusOK {
		t.Fatalf("expected validate-key to return %d, got %d", http.StatusOK, status)
	}
	if !result.Valid {
		t.Fatalf("expected a freshly created key to be valid, got reason %q", result.Reason)
	}
	if result.Owner != "validate-owner" {
		t.Fatalf("expected owner %q, got %q", "validate-owner", result.Owner)
	}
}

func TestApp_ValidateKeyToleratesSurroundingWhitespace(t *testing.T) {
	server := startServer(t)
	created := createTestKey(t, server, `{"owner":"validate-owner"}`)

	status, result := validateTestKey(t, server, "  "+created.Key+"\n")

	if status != http.StatusOK {
		t.Fatalf("expected validate-key to return %d, got %d", http.StatusOK, status)
	}
	if !result.Valid {
		t.Fatalf("expected a valid key with surrounding whitespace to still validate, got reason %q", result.Reason)
	}
}

func TestApp_ValidateKeyRejectsAWrongSecret(t *testing.T) {
	server := startServer(t)
	created := createTestKey(t, server, `{"owner":"validate-owner"}`)

	tampered := created.Key[:len(created.Key)-1] + "0"
	if tampered == created.Key {
		tampered = created.Key[:len(created.Key)-1] + "1"
	}

	status, result := validateTestKey(t, server, tampered)

	if status != http.StatusOK {
		t.Fatalf("expected validate-key to return %d, got %d", http.StatusOK, status)
	}
	if result.Valid {
		t.Fatal("expected a key with a tampered secret to be invalid")
	}
	if result.Reason != "invalid key" {
		t.Fatalf("expected a generic invalid-key reason, got %q", result.Reason)
	}
}

func TestApp_ValidateKeyRejectsAnUnknownKeyID(t *testing.T) {
	server := startServer(t)

	status, result := validateTestKey(t, server, "api_00000000_"+strings.Repeat("a", 32))

	if status != http.StatusOK {
		t.Fatalf("expected validate-key to return %d, got %d", http.StatusOK, status)
	}
	if result.Valid {
		t.Fatal("expected an unknown key id to be invalid")
	}
	if result.Reason != "invalid key" {
		t.Fatalf("expected a generic invalid-key reason, got %q", result.Reason)
	}
}

func TestApp_ValidateKeyRejectsAMalformedKey(t *testing.T) {
	server := startServer(t)

	status, result := validateTestKey(t, server, "not-a-key")

	if status != http.StatusOK {
		t.Fatalf("expected validate-key to return %d, got %d", http.StatusOK, status)
	}
	if result.Valid {
		t.Fatal("expected a malformed key to be invalid")
	}
	if result.Reason != "invalid key" {
		t.Fatalf("expected a generic invalid-key reason, got %q", result.Reason)
	}
}

func TestApp_ValidateKeyRejectsAnExpiredKey(t *testing.T) {
	server := startServer(t)
	created := createTestKey(t, server, `{"owner":"validate-owner","expires_at":"2020-01-01T00:00:00Z"}`)

	status, result := validateTestKey(t, server, created.Key)

	if status != http.StatusOK {
		t.Fatalf("expected validate-key to return %d, got %d", http.StatusOK, status)
	}
	if result.Valid {
		t.Fatal("expected an expired key to be invalid")
	}
	if result.Reason != "key has expired" {
		t.Fatalf("expected reason %q, got %q", "key has expired", result.Reason)
	}
}

func TestApp_ValidateKeyRequiresAKey(t *testing.T) {
	server := startServer(t)

	resp, err := http.Post(server.URL+"/api/keys/validate", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("calling validate-key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected validate-key without a key to return %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestNew_FailsFastWhenDatabaseIsUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := app.New(ctx, config.Config{
		DatabaseURL: "postgres://user:pass@127.0.0.1:1/nonexistent?sslmode=disable",
	})
	if err == nil {
		t.Fatal("expected an error when the database is unreachable, got nil")
	}
}
