// Package migrations embeds the versioned SQL migrations so the migrate command
// carries them inside its binary. Nothing here runs during request handling.
package migrations

import "embed"

// FS holds every versioned migration, ordered by its numeric filename prefix.
//
//go:embed *.sql
var FS embed.FS
