CREATE TABLE keys (
    key_id text PRIMARY KEY,
    secret_hash text NOT NULL,
    owner text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz
);
