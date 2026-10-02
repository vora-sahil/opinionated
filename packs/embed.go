// Package packs embeds the packs that ship inside the opine binary.
package packs

import "embed"

//go:embed all:universal
var FS embed.FS
