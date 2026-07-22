package migrations

import "embed"

// FS contains all application SQL migrations for the single SQLite database.
//
// FS contains embedded database migrations.
//
//go:embed *.sql
var FS embed.FS
