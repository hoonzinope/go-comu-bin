package sqlite

import "embed"

//go:embed migrations/*.sql
var embeddedMigrations embed.FS
