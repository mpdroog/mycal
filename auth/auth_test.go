package auth

import (
	"errors"
	"os"
	"testing"

	"github.com/mpdroog/mycal/db"
)

func TestMain(m *testing.M) {
	// Create temp directory for test database
	tmpDir, err := os.MkdirTemp("", "mycal-auth-test-*")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(tmpDir)

	// Initialize test database
	if err := db.Init(tmpDir); err != nil {
		panic(err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			panic(err)
		}
	}()

	os.Exit(m.Run())
}

func TestHashPassword(t *testing.T) {
	password := "testpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == password {
		t.Error("Hash should not equal plain password")
	}

	if len(hash) < 50 {
		t.Error("Hash seems too short for bcrypt")
	}
}

func TestCheckPassword(t *testing.T) {
	password := "testpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if !CheckPassword(hash, password) {
		t.Error("CheckPassword should return true for correct password")
	}

	if CheckPassword(hash, "wrongpassword") {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestGenerateSessionID(t *testing.T) {
	id1, err := GenerateSessionID()
	if err != nil {
		t.Fatalf("GenerateSessionID failed: %v", err)
	}

	if len(id1) != 64 {
		t.Errorf("Session ID should be 64 hex chars, got %d", len(id1))
	}

	id2, err := GenerateSessionID()
	if err != nil {
		t.Fatalf("GenerateSessionID failed: %v", err)
	}

	if id1 == id2 {
		t.Error("Session IDs should be unique")
	}
}

func TestCreateUser(t *testing.T) {
	user, err := CreateUser("testuser1", "password123", false)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if user.ID == 0 {
		t.Error("User should have an ID")
	}

	if user.Username != "testuser1" {
		t.Errorf("Username mismatch: got %s, want testuser1", user.Username)
	}

	if user.IsAdmin {
		t.Error("User should not be admin")
	}
}

func TestCreateUserDuplicate(t *testing.T) {
	_, err := CreateUser("duplicateuser", "password123", false)
	if err != nil {
		t.Fatalf("First CreateUser failed: %v", err)
	}

	_, err = CreateUser("duplicateuser", "password456", false)
	if !errors.Is(err, ErrUserExists) {
		t.Errorf("Expected ErrUserExists, got: %v", err)
	}
}

func TestCreateUserAdmin(t *testing.T) {
	user, err := CreateUser("adminuser1", "password123", true)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if !user.IsAdmin {
		t.Error("User should be admin")
	}
}

func TestGetUserByUsername(t *testing.T) {
	_, err := CreateUser("getbyusername", "password123", false)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	user, err := GetUserByUsername("getbyusername")
	if err != nil {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}

	if user.Username != "getbyusername" {
		t.Errorf("Username mismatch: got %s", user.Username)
	}

	// Should have password hash
	if user.PasswordHash == "" {
		t.Error("PasswordHash should be set")
	}
}

func TestGetUserByUsernameNotFound(t *testing.T) {
	_, err := GetUserByUsername("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent user")
	}
}

func TestGetUserByID(t *testing.T) {
	created, err := CreateUser("getbyid", "password123", false)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	user, err := GetUserByID(created.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}

	if user.ID != created.ID {
		t.Errorf("ID mismatch: got %d, want %d", user.ID, created.ID)
	}
}

func TestCreateSession(t *testing.T) {
	user, err := CreateUser("sessionuser", "password123", false)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	sessionID, err := CreateSession(user.ID)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if len(sessionID) != 64 {
		t.Errorf("Session ID should be 64 hex chars, got %d", len(sessionID))
	}
}

func TestGetUserFromSession(t *testing.T) {
	user, err := CreateUser("sessionlookup", "password123", false)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	sessionID, err := CreateSession(user.ID)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	retrieved, err := GetUserFromSession(sessionID)
	if err != nil {
		t.Fatalf("GetUserFromSession failed: %v", err)
	}

	if retrieved.ID != user.ID {
		t.Errorf("User ID mismatch: got %d, want %d", retrieved.ID, user.ID)
	}

	if retrieved.Username != user.Username {
		t.Errorf("Username mismatch: got %s, want %s", retrieved.Username, user.Username)
	}
}

func TestGetUserFromSessionInvalid(t *testing.T) {
	_, err := GetUserFromSession("invalidsessionid")
	if err == nil {
		t.Error("Expected error for invalid session")
	}
}

func TestDeleteSession(t *testing.T) {
	user, err := CreateUser("deletesession", "password123", false)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	sessionID, err := CreateSession(user.ID)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Verify session works
	_, err = GetUserFromSession(sessionID)
	if err != nil {
		t.Fatalf("GetUserFromSession failed before delete: %v", err)
	}

	// Delete session
	err = DeleteSession(sessionID)
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// Session should no longer work
	_, err = GetUserFromSession(sessionID)
	if err == nil {
		t.Error("Session should be deleted")
	}
}

func TestHasUsers(t *testing.T) {
	// We've already created users in previous tests
	has, err := HasUsers()
	if err != nil {
		t.Fatalf("HasUsers failed: %v", err)
	}

	if !has {
		t.Error("HasUsers should return true after creating users")
	}
}

func TestGetAllUsers(t *testing.T) {
	users, err := GetAllUsers()
	if err != nil {
		t.Fatalf("GetAllUsers failed: %v", err)
	}

	if len(users) == 0 {
		t.Error("Should have at least one user")
	}

	// Verify users don't have password hash exposed
	for _, u := range users {
		if u.PasswordHash != "" {
			t.Error("GetAllUsers should not return password hashes")
		}
	}
}

func TestUpdateUserPassword(t *testing.T) {
	user, err := CreateUser("updatepwd", "oldpassword", false)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	err = UpdateUserPassword(user.ID, "newpassword")
	if err != nil {
		t.Fatalf("UpdateUserPassword failed: %v", err)
	}

	// Verify old password doesn't work
	updated, err := GetUserByUsername("updatepwd")
	if err != nil {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}

	if CheckPassword(updated.PasswordHash, "oldpassword") {
		t.Error("Old password should not work")
	}

	if !CheckPassword(updated.PasswordHash, "newpassword") {
		t.Error("New password should work")
	}
}

func TestUpdateUserAdmin(t *testing.T) {
	user, err := CreateUser("updateadmin", "password123", false)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if user.IsAdmin {
		t.Error("User should not be admin initially")
	}

	err = UpdateUserAdmin(user.ID, true)
	if err != nil {
		t.Fatalf("UpdateUserAdmin failed: %v", err)
	}

	updated, err := GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}

	if !updated.IsAdmin {
		t.Error("User should be admin after update")
	}
}

func TestDeleteUser(t *testing.T) {
	user, err := CreateUser("deleteuser", "password123", false)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Create a session for the user
	_, err = CreateSession(user.ID)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	err = DeleteUser(user.ID)
	if err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	// User should no longer exist
	_, err = GetUserByID(user.ID)
	if err == nil {
		t.Error("User should be deleted")
	}
}

func TestCountAdmins(t *testing.T) {
	// Create a known admin user
	_, err := CreateUser("countadmin1", "password123", true)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	count, err := CountAdmins()
	if err != nil {
		t.Fatalf("CountAdmins failed: %v", err)
	}

	if count < 1 {
		t.Error("Should have at least 1 admin")
	}
}

func TestIsAdmin(t *testing.T) {
	admin, err := CreateUser("isadmintest", "password123", true)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	nonAdmin, err := CreateUser("isnonadmintest", "password123", false)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	isAdmin, err := IsAdmin(admin.ID)
	if err != nil {
		t.Fatalf("IsAdmin failed: %v", err)
	}

	if !isAdmin {
		t.Error("Admin user should return true")
	}

	isAdmin, err = IsAdmin(nonAdmin.ID)
	if err != nil {
		t.Fatalf("IsAdmin failed: %v", err)
	}

	if isAdmin {
		t.Error("Non-admin user should return false")
	}
}
