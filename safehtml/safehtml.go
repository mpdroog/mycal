// Package safehtml provides HTML-safe string formatting.
// All string arguments are HTML-escaped by default.
// Use UnsafeHTML() to bypass escaping for pre-escaped content.
package safehtml

import (
	"fmt"
	"html"
	"strings"
)

// UnsafeHTML marks a string as already HTML-safe (pre-escaped or trusted markup).
// Use sparingly - only for content you control or have already escaped.
// The name is intentionally scary to discourage misuse.
type UnsafeHTML string

// Sprintf formats according to a format specifier and returns the resulting string.
// All string arguments are HTML-escaped unless wrapped in UnsafeHTML().
//
// Example:
//
//	safehtml.Sprintf("<div>%s</div>", userInput)               // userInput is escaped
//	safehtml.Sprintf("<div>%s</div>", safehtml.UnsafeHTML(x))  // x is NOT escaped
func Sprintf(format string, args ...interface{}) string {
	escaped := make([]interface{}, len(args))

	for i, arg := range args {
		switch v := arg.(type) {
		case UnsafeHTML:
			// Explicitly marked as pre-escaped, use as-is
			escaped[i] = string(v)
		case string:
			// Escape HTML in strings
			escaped[i] = html.EscapeString(v)
		case fmt.Stringer:
			// Escape anything that converts to string
			escaped[i] = html.EscapeString(v.String())
		default:
			// Numbers, bools, etc. are safe
			escaped[i] = arg
		}
	}

	return fmt.Sprintf(format, escaped...)
}

// Escape returns an HTML-escaped version of the string.
func Escape(s string) string {
	return html.EscapeString(s)
}

// Join joins strings with a separator, escaping each element.
// The separator is NOT escaped.
func Join(elems []string, sep string) UnsafeHTML {
	escaped := make([]string, len(elems))
	for i, elem := range elems {
		escaped[i] = html.EscapeString(elem)
	}

	return UnsafeHTML(strings.Join(escaped, sep))
}
