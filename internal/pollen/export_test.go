package pollen

import (
	"airbg.org/internal/config"
	"airbg.org/internal/store"
)

// RequestURLForTesting exposes the URL builder to the external test package.
func RequestURLForTesting(cfg config.Pollen, cells []store.PollenCell) string {
	return New(cfg).requestURL(cells)
}
