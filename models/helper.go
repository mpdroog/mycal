package models

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// normalizeName lowercases and removes accents for consistent trigram matching.
func normalizeName(s string) string {
	// Remove accents
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

	result, _, err := transform.String(t, s)
	if err != nil {
		// Fall back to original string if transformation fails
		return strings.ToLower(s)
	}

	return strings.ToLower(result)
}
