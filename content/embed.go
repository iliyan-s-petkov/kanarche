// Package content embeds the published Markdown posts of the /news section.
// _drafts directories are skipped by go:embed, so drafts never ship.
package content

import "embed"

//go:embed news
var News embed.FS
