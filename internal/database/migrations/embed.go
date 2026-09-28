// Package migrations holds the embedded SQL migration files applied on startup.
package migrations

import "embed"

// FS holds the embedded SQL migration files, applied on startup via golang-migrate.
//
//go:embed *.sql
var FS embed.FS
