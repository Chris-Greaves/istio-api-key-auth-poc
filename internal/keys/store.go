package keys

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// Record is a key's metadata as stored and surfaced to callers — it never
// carries the secret or its hash.
type Record struct {
	KeyID     string
	Owner     string
	CreatedAt time.Time
	ExpiresAt *time.Time
}

// ErrKeyIDCollision indicates the generated Key ID already exists. Callers
// should generate a new key and retry.
var ErrKeyIDCollision = errors.New("key id already exists")

// Store persists API key metadata in Postgres.
type Store struct {
	db *sql.DB
}

// NewStore builds a Store backed by db.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new key's metadata and secret hash.
func (s *Store) Create(ctx context.Context, keyID, secretHash, owner string, expiresAt *time.Time) (Record, error) {
	var rec Record
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO keys (key_id, secret_hash, owner, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING key_id, owner, created_at, expires_at
	`, keyID, secretHash, owner, expiresAt).Scan(&rec.KeyID, &rec.Owner, &rec.CreatedAt, &rec.ExpiresAt)
	if err != nil {
		if isKeyIDCollision(err) {
			return Record{}, ErrKeyIDCollision
		}
		return Record{}, fmt.Errorf("inserting key: %w", err)
	}
	return rec, nil
}

// List returns metadata for every key, most recently created first.
func (s *Store) List(ctx context.Context) ([]Record, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT key_id, owner, created_at, expires_at FROM keys ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying keys: %w", err)
	}
	defer rows.Close()

	records := []Record{}
	for rows.Next() {
		var rec Record
		if err := rows.Scan(&rec.KeyID, &rec.Owner, &rec.CreatedAt, &rec.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scanning key row: %w", err)
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating key rows: %w", err)
	}
	return records, nil
}

// isKeyIDCollision reports whether err is a violation of the keys table's
// key_id primary key specifically — not any unique constraint — so a future
// unique constraint on another column isn't misdiagnosed as a Key ID clash.
func isKeyIDCollision(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == "23505" && pgErr.ConstraintName == "keys_pkey"
}
