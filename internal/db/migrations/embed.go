// Package migrations carries the goose migration files atlas generates.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
