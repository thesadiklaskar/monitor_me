package migrations

import "embed"

// Files embeds the versioned SQL migrations into the migration runner binary.
// This keeps migration execution independent of the container working directory.
//
//go:embed *.sql
var Files embed.FS
