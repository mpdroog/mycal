package auth

import (
	"context"
	"net/http"

	"github.com/mpdroog/mycal/models"
)

type contextKey string

const UserContextKey contextKey = "user"

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
