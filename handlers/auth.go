package handlers

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/mpdroog/mycal/auth"
	"github.com/mpdroog/mycal/db"
)

// Setup handles first-run admin creation.
func Setup(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Block access if setup already completed
		hasUsers, err := auth.HasUsers()
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if hasUsers {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if r.Method == http.MethodGet {
			data := map[string]interface{}{
				"Title": "Setup",
			}

			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				httpError(w, err, http.StatusInternalServerError)
			}

			return
		}

		// POST - create first admin
		if err := r.ParseForm(); err != nil {
			httpError(w, err, http.StatusBadRequest)

			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")

		if username == "" || password == "" {
			data := map[string]interface{}{
				"Title": "Setup",
				"Error": "Username and password are required",
			}

			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				httpError(w, err, http.StatusInternalServerError)
			}

			return
		}

		if password != confirmPassword {
			data := map[string]interface{}{
				"Title": "Setup",
				"Error": "Passwords do not match",
			}

			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				httpError(w, err, http.StatusInternalServerError)
			}

			return
		}

		if len(password) < 8 {
			data := map[string]interface{}{
				"Title": "Setup",
				"Error": "Password must be at least 8 characters",
			}

			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				httpError(w, err, http.StatusInternalServerError)
			}

			return
		}

		// Create admin user
		user, err := auth.CreateUser(username, password, true)
		if err != nil {
			data := map[string]interface{}{
				"Title": "Setup",
				"Error": "Failed to create user: " + err.Error(),
			}

			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				httpError(w, err, http.StatusInternalServerError)
			}

			return
		}

		// Assign orphaned data to this user
		_ = auth.AssignOrphanedData(user.ID)

		// Create default profile for user if none exists
		db.DB.Exec(`INSERT OR IGNORE INTO profile (user_id, calories_goal, protein_goal, carbs_goal, fat_goal)
			VALUES (?, 2000, 150, 250, 65)`, user.ID)

		// Create session and login
		sessionID, err := auth.CreateSession(user.ID)
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)

			return
		}

		auth.SetSessionCookie(w, sessionID)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// Login handles user authentication.
func Login(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			data := map[string]interface{}{
				"Title": "Login",
			}

			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				httpError(w, err, http.StatusInternalServerError)
			}

			return
		}

		// POST - authenticate
		if err := r.ParseForm(); err != nil {
			httpError(w, err, http.StatusBadRequest)

			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		user, err := auth.GetUserByUsername(username)
		if err != nil || !auth.CheckPassword(user.PasswordHash, password) {
			data := map[string]interface{}{
				"Title": "Login",
				"Error": "Invalid username or password",
			}

			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				httpError(w, err, http.StatusInternalServerError)
			}

			return
		}

		sessionID, err := auth.CreateSession(user.ID)
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)

			return
		}

		auth.SetSessionCookie(w, sessionID)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// Logout handles user logout.
func Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.CookieName)
	if err == nil {
		_ = auth.DeleteSession(cookie.Value)
	}

	auth.ClearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// AdminUsers handles user management list.
func AdminUsers(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := auth.GetAllUsers()
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)

			return
		}

		currentUser := auth.GetUserFromContext(r.Context())

		data := map[string]interface{}{
			"Title": "Users",
			"Users": users,
			"User":  currentUser,
		}

		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			httpError(w, err, http.StatusInternalServerError)
		}
	}
}

// AdminCreateUser handles admin user creation.
func AdminCreateUser(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser := auth.GetUserFromContext(r.Context())

		if r.Method == http.MethodGet {
			data := map[string]interface{}{
				"Title": "Create User",
				"User":  currentUser,
			}

			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				httpError(w, err, http.StatusInternalServerError)
			}

			return
		}

		// POST
		if err := r.ParseForm(); err != nil {
			httpError(w, err, http.StatusBadRequest)

			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")
		isAdmin := r.FormValue("is_admin") == "on"

		if username == "" || password == "" {
			data := map[string]interface{}{
				"Title": "Create User",
				"User":  currentUser,
				"Error": "Username and password are required",
			}

			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				httpError(w, err, http.StatusInternalServerError)
			}

			return
		}

		if len(password) < 8 {
			data := map[string]interface{}{
				"Title": "Create User",
				"User":  currentUser,
				"Error": "Password must be at least 8 characters",
			}

			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				httpError(w, err, http.StatusInternalServerError)
			}

			return
		}

		user, err := auth.CreateUser(username, password, isAdmin)
		if err != nil {
			data := map[string]interface{}{
				"Title": "Create User",
				"User":  currentUser,
				"Error": "Failed to create user: " + err.Error(),
			}

			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				httpError(w, err, http.StatusInternalServerError)
			}

			return
		}

		// Create default profile for new user
		db.DB.Exec(`INSERT INTO profile (user_id, calories_goal, protein_goal, carbs_goal, fat_goal)
			VALUES (?, 2000, 150, 250, 65)`, user.ID)

		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
	}
}

// AdminEditUser handles admin user editing.
func AdminEditUser(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser := auth.GetUserFromContext(r.Context())

		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)

			return
		}

		editUser, err := auth.GetUserByID(id)
		if err != nil {
			http.NotFound(w, r)

			return
		}

		if r.Method == http.MethodGet {
			data := map[string]interface{}{
				"Title":    "Edit User",
				"User":     currentUser,
				"EditUser": editUser,
			}

			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				httpError(w, err, http.StatusInternalServerError)
			}

			return
		}

		// POST
		if err := r.ParseForm(); err != nil {
			httpError(w, err, http.StatusBadRequest)

			return
		}

		password := r.FormValue("password")
		isAdmin := r.FormValue("is_admin") == "on"

		// Update password if provided
		if password != "" {
			if len(password) < 8 {
				data := map[string]interface{}{
					"Title":    "Edit User",
					"User":     currentUser,
					"EditUser": editUser,
					"Error":    "Password must be at least 8 characters",
				}

				if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
					httpError(w, err, http.StatusInternalServerError)
				}

				return
			}

			if err := auth.UpdateUserPassword(id, password); err != nil {
				httpError(w, err, http.StatusInternalServerError)

				return
			}
		}

		// Update admin status
		if err := auth.UpdateUserAdmin(id, isAdmin); err != nil {
			httpError(w, err, http.StatusInternalServerError)

			return
		}

		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
	}
}

// AdminDeleteUser handles admin user deletion.
func AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	currentUser := auth.GetUserFromContext(r.Context())

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)

		return
	}

	// Prevent self-deletion
	if id == currentUser.ID {
		http.Error(w, "Cannot delete your own account", http.StatusBadRequest)

		return
	}

	// Prevent deleting the last admin
	isAdmin, err := auth.IsAdmin(id)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)

		return
	}

	if isAdmin {
		adminCount, err := auth.CountAdmins()
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)

			return
		}

		if adminCount <= 1 {
			http.Error(w, "Cannot delete the last admin account", http.StatusBadRequest)

			return
		}
	}

	if err := auth.DeleteUser(id); err != nil {
		httpError(w, err, http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}
