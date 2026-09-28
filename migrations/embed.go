package migrations

import "embed"

// Files contains ordered up and down migration scripts.
//
//go:embed *.sql
var Files embed.FS
