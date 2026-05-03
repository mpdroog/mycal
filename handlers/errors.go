package handlers

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"runtime"
)

// httpError logs the actual error with location and returns a generic message to the user.
// The user sees a reference like "Internal error [auth.go:42]" for support purposes.
func httpError(w http.ResponseWriter, err error, code int) {
	_, file, line, _ := runtime.Caller(1)
	file = filepath.Base(file)
	ref := fmt.Sprintf("[%s:%d]", file, line)

	log.Printf("HTTP %d %s: %v", code, ref, err)

	switch code {
	case http.StatusBadRequest:
		http.Error(w, "Bad request "+ref, code)
	case http.StatusNotFound:
		http.Error(w, "Not found "+ref, code)
	case http.StatusConflict:
		http.Error(w, "Conflict "+ref, code)
	case http.StatusForbidden:
		http.Error(w, "Forbidden "+ref, code)
	default:
		http.Error(w, "Internal error "+ref, code)
	}
}

// renderTemplate executes the base template and logs any error.
func renderTemplate(w http.ResponseWriter, tmpl *template.Template, data map[string]interface{}) {
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}
