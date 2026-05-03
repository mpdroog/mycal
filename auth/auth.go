package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/mpdroog/mycal/db"
	"github.com/mpdroog/mycal/models"
)

const (
	BcryptCost      = 12
	SessionDuration = 30 * 24 * time.Hour // 30 days
	CookieName      = "session"
)

// SecureCookie controls whether session cookies require HTTPS.
// Defaults to true for production safety.
var SecureCookie = true

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserExists         = errors.New("username already exists")
)

// HashPassword creates a bcrypt hash.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	return string(hash), err
}

// CheckPassword verifies a password against hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GenerateSessionID creates a secure random session ID.
func GenerateSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

// CreateUser creates a new user.
func CreateUser(username, password string, isAdmin bool) (*models.User, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	result, err := db.DB.Exec(
		`INSERT INTO users (username, password_hash, is_admin) VALUES (?, ?, ?)`,
		username, hash, isAdmin,
	)
	if err != nil {
		return nil, ErrUserExists
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &models.User{ID: id, Username: username, IsAdmin: isAdmin, CreatedAt: time.Now()}, nil
}

// GetUserByUsername retrieves user by username.
func GetUserByUsername(username string) (*models.User, error) {
	var u models.User

	err := db.DB.QueryRow(
		`SELECT id, username, password_hash, is_admin, created_at FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

// GetUserByID retrieves user by ID.
func GetUserByID(id int64) (*models.User, error) {
	var u models.User

	err := db.DB.QueryRow(
		`SELECT id, username, password_hash, is_admin, created_at FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

// CreateSession creates a new session for user.
func CreateSession(userID int64) (string, error) {
	sessionID, err := GenerateSessionID()
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().Add(SessionDuration)
	_, err = db.DB.Exec(
		`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`,
		sessionID, userID, expiresAt,
	)
	if err != nil {
		return "", err
	}

	return sessionID, nil
}

// GetUserFromSession retrieves user from session ID.
func GetUserFromSession(sessionID string) (*models.User, error) {
	var userID int64

	err := db.DB.QueryRow(
		`SELECT user_id FROM sessions WHERE id = ? AND expires_at > ?`,
		sessionID, time.Now(),
	).Scan(&userID)
	if err != nil {
		return nil, err
	}

	return GetUserByID(userID)
}

// DeleteSession removes a session (logout).
func DeleteSession(sessionID string) error {
	_, err := db.DB.Exec(`DELETE FROM sessions WHERE id = ?`, sessionID)
	return err
}

// CleanExpiredSessions removes expired sessions.
func CleanExpiredSessions() error {
	_, err := db.DB.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now())
	return err
}

// HasUsers checks if any users exist.
func HasUsers() (bool, error) {
	var count int

	err := db.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)

	return count > 0, err
}

// GetAllUsers returns all users (for admin panel).
func GetAllUsers() ([]models.User, error) {
	rows, err := db.DB.Query(`SELECT id, username, is_admin, created_at FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User

	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt); err != nil {
			return nil, err
		}

		users = append(users, u)
	}

	return users, rows.Err()
}

// SetSessionCookie sets the session cookie on response.
func SetSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   SecureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(SessionDuration.Seconds()),
	})
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// AssignOrphanedData assigns entries and profile with NULL user_id to a user.
func AssignOrphanedData(userID int64) error {
	_, err := db.DB.Exec("UPDATE entries SET user_id = ? WHERE user_id IS NULL", userID)
	if err != nil {
		return err
	}

	_, err = db.DB.Exec("UPDATE profile SET user_id = ? WHERE user_id IS NULL", userID)

	return err
}

// UpdateUserPassword updates a user's password.
func UpdateUserPassword(userID int64, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}

	_, err = db.DB.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, userID)

	return err
}

// UpdateUserAdmin updates a user's admin status.
func UpdateUserAdmin(userID int64, isAdmin bool) error {
	_, err := db.DB.Exec(`UPDATE users SET is_admin = ? WHERE id = ?`, isAdmin, userID)
	return err
}

// CountAdmins returns the number of admin users.
func CountAdmins() (int, error) {
	var count int

	err := db.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE is_admin = 1`).Scan(&count)

	return count, err
}

// IsAdmin checks if a user is an admin.
func IsAdmin(userID int64) (bool, error) {
	var isAdmin bool

	err := db.DB.QueryRow(`SELECT is_admin FROM users WHERE id = ?`, userID).Scan(&isAdmin)

	return isAdmin, err
}

// DeleteUser deletes a user and their data.
func DeleteUser(userID int64) error {
	// Delete user's entries
	_, err := db.DB.Exec(`DELETE FROM entries WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}

	// Delete user's profile
	_, err = db.DB.Exec(`DELETE FROM profile WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}

	// Delete user's sessions
	_, err = db.DB.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}

	// Delete user
	_, err = db.DB.Exec(`DELETE FROM users WHERE id = ?`, userID)

	return err
}
