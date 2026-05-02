package auth

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/mpdroog/mycal/models"
)

type contextKey string

const UserContextKey contextKey = "user"

// CheckCSRF middleware validates Origin/Referer headers for state-changing requests.
// This provides CSRF protection by ensuring requests originate from the same site.
func CheckCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only check state-changing methods
		if r.Method != http.MethodPost && r.Method != http.MethodPut &&
			r.Method != http.MethodDelete && r.Method != http.MethodPatch {
			next.ServeHTTP(w, r)
			return
		}

		// Get the expected host from the request
		expectedHost := r.Host

		// Check Origin header first (preferred)
		origin := r.Header.Get("Origin")
		if origin != "" {
			originURL, err := url.Parse(origin)
			if err != nil || originURL.Host != expectedHost {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		// Fall back to Referer header
		referer := r.Header.Get("Referer")
		if referer != "" {
			refererURL, err := url.Parse(referer)
			if err != nil || refererURL.Host != expectedHost {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		// Neither Origin nor Referer present.
		// With SameSite=Lax cookies, cross-site POST requests won't have the session cookie,
		// but we should still reject requests without origin info for defense-in-depth.
		// Exception: allow if it looks like a same-origin request (no origin typically means same-origin in older browsers)
		// Check if this is likely a direct form submission (Content-Type form)
		contentType := r.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") ||
			strings.HasPrefix(contentType, "multipart/form-data") {
			// Browser form submissions from same origin may omit Origin/Referer in some cases
			// SameSite=Lax provides protection here, so we allow it
			next.ServeHTTP(w, r)
			return
		}

		// For other requests without Origin/Referer (e.g., fetch/XHR), reject
		http.Error(w, "Forbidden", http.StatusForbidden)
	})
}

// RequireAuth middleware ensures user is logged in.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(CookieName)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)

			return
		}

		user, err := GetUserFromSession(cookie.Value)
		if err != nil {
			ClearSessionCookie(w)
			http.Redirect(w, r, "/login", http.StatusSeeOther)

			return
		}

		// Add user to context
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin middleware ensures user is admin.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user == nil || !user.IsAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)

			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireSetup middleware redirects to setup if no users exist.
func RequireSetup(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasUsers, err := HasUsers()
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)

			return
		}

		if !hasUsers && r.URL.Path != "/setup" {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)

			return
		}

		if hasUsers && r.URL.Path == "/setup" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)

			return
		}

		next.ServeHTTP(w, r)
	})
}

// GetUserFromContext retrieves user from request context.
func GetUserFromContext(ctx context.Context) *models.User {
	user, ok := ctx.Value(UserContextKey).(*models.User)
	if !ok {
		return nil
	}

	return user
}
