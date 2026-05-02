package main

import (
	"context"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mpdroog/mycal/auth"
	"github.com/mpdroog/mycal/db"
	"github.com/mpdroog/mycal/handlers"
	"github.com/mpdroog/mycal/models"
)

var testServer *httptest.Server
var testUser *models.User

func TestMain(m *testing.M) {
	// Create temp directory for test database
	tmpDir, err := os.MkdirTemp("", "mycal-main-test-*")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(tmpDir)

	// Initialize test database
	initErr := db.Init(tmpDir)
	if initErr != nil {
		panic(initErr)
	}

	defer func() {
		closeErr := db.Close()
		if closeErr != nil {
			panic(closeErr)
		}
	}()

	// Create a test user for authentication
	testUser, err = auth.CreateUser("testuser", "testpass123", true)
	if err != nil {
		panic(err)
	}

	// Create profile for test user
	_, _ = db.DB.Exec(`INSERT INTO profile (user_id, calories_goal, protein_goal, carbs_goal, fat_goal)
		VALUES (?, 2000, 150, 250, 65)`, testUser.ID)

	// Setup templates and server
	tmpls, loadErr := loadTestTemplates()
	if loadErr != nil {
		panic(loadErr)
	}

	r := setupRouter(tmpls)
	testServer = httptest.NewServer(r)

	defer testServer.Close()

	os.Exit(m.Run())
}

func loadTestTemplates() (*Templates, error) {
	funcMap := template.FuncMap{
		"prevDay": func(date string) string {
			t, err := time.Parse("2006-01-02", date)
			if err != nil {
				return date
			}

			return t.AddDate(0, 0, -1).Format("2006-01-02")
		},
		"nextDay": func(date string) string {
			t, err := time.Parse("2006-01-02", date)
			if err != nil {
				return date
			}

			return t.AddDate(0, 0, 1).Format("2006-01-02")
		},
		"relativeDate": func(date string) string {
			t, err := time.Parse("2006-01-02", date)
			if err != nil {
				return ""
			}

			now := time.Now()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			target := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, now.Location())

			days := int(target.Sub(today).Hours() / 24)

			switch days {
			case 0:
				return "Today"
			case 1:
				return "Tomorrow"
			case -1:
				return "Yesterday"
			}

			weekday := t.Weekday().String()

			if days >= 2 && days <= 6 {
				return weekday
			}

			if days >= -6 && days <= -2 {
				return "Last " + weekday
			}

			if days > 6 && days <= 13 {
				return "Next " + weekday
			}

			weeks := days / 7
			if days < 0 {
				weeks = (-days) / 7
				if weeks == 1 {
					return weekday + ", 1 week ago"
				}

				return weekday + ", " + strconv.Itoa(weeks) + " weeks ago"
			}

			if weeks == 1 {
				return weekday + ", in 1 week"
			}

			return weekday + ", in " + strconv.Itoa(weeks) + " weeks"
		},
		"title": cases.Title(language.English).String,
		"multiply": func(a int, b float64) int {
			return int(float64(a) * b)
		},
		"divide": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}

			return a / b
		},
		"percentage": func(value, goal float64) int {
			if goal == 0 {
				return 0
			}

			pct := (value / goal) * 100
			if pct > 100 {
				return 100
			}

			return int(pct)
		},
		"intToFloat": func(i int) float64 {
			return float64(i)
		},
		"multiplyFloat": func(a, b float64) float64 {
			return a * b
		},
	}

	base := filepath.Join("templates", "base.html")

	dashboard, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join("templates", "dashboard.html"))
	if err != nil {
		return nil, err
	}

	ingredients, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join("templates", "ingredients.html"))
	if err != nil {
		return nil, err
	}

	ingredientForm, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join("templates", "ingredient_form.html"))
	if err != nil {
		return nil, err
	}

	foods, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join("templates", "foods.html"))
	if err != nil {
		return nil, err
	}

	foodForm, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join("templates", "food_form.html"))
	if err != nil {
		return nil, err
	}

	entryForm, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join("templates", "entry_form.html"))
	if err != nil {
		return nil, err
	}

	profile, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join("templates", "profile.html"))
	if err != nil {
		return nil, err
	}

	login, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join("templates", "login.html"))
	if err != nil {
		return nil, err
	}

	setup, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join("templates", "setup.html"))
	if err != nil {
		return nil, err
	}

	adminUsers, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join("templates", "admin_users.html"))
	if err != nil {
		return nil, err
	}

	adminUserForm, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join("templates", "admin_user_form.html"))
	if err != nil {
		return nil, err
	}

	return &Templates{
		Dashboard:      dashboard,
		Ingredients:    ingredients,
		IngredientForm: ingredientForm,
		Foods:          foods,
		FoodForm:       foodForm,
		EntryForm:      entryForm,
		Profile:        profile,
		Login:          login,
		Setup:          setup,
		AdminUsers:     adminUsers,
		AdminUserForm:  adminUserForm,
	}, nil
}

func setupRouter(tmpls *Templates) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	// Inject test user into context for all requests
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), auth.UserContextKey, testUser)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	fs := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	// Public routes (no auth required)
	r.Get("/login", handlers.Login(tmpls.Login))
	r.Post("/login", handlers.Login(tmpls.Login))

	r.Get("/", handlers.Dashboard(tmpls.Dashboard))
	r.Post("/logout", handlers.Logout)

	// Ingredients
	r.Get("/ingredients", handlers.ListIngredients(tmpls.Ingredients))
	r.Get("/ingredients/new", handlers.CreateIngredient(tmpls.IngredientForm))
	r.Post("/ingredients/new", handlers.CreateIngredient(tmpls.IngredientForm))
	r.Get("/ingredients/{id}/edit", handlers.EditIngredient(tmpls.IngredientForm))
	r.Post("/ingredients/{id}/edit", handlers.EditIngredient(tmpls.IngredientForm))
	r.Post("/ingredients/{id}/delete", handlers.DeleteIngredient)
	r.Get("/ingredients/search", handlers.SearchIngredients)

	// Foods
	r.Get("/foods", handlers.ListFoods(tmpls.Foods))
	r.Get("/foods/new", handlers.CreateFood(tmpls.FoodForm))
	r.Post("/foods/new", handlers.CreateFood(tmpls.FoodForm))
	r.Get("/foods/{id}/edit", handlers.EditFood(tmpls.FoodForm))
	r.Post("/foods/{id}/edit", handlers.EditFood(tmpls.FoodForm))
	r.Post("/foods/{id}/delete", handlers.DeleteFood)

	// Entries
	r.Post("/entries", handlers.CreateEntry)
	r.Get("/entries/{id}/edit", handlers.GetEntry(tmpls.EntryForm))
	r.Post("/entries/{id}/edit", handlers.UpdateEntry)
	r.Post("/entries/{id}/servings", handlers.UpdateEntryServings)
	r.Post("/entries/{id}/delete", handlers.DeleteEntry)

	// Profile
	r.Get("/profile", handlers.Profile(tmpls.Profile))
	r.Post("/profile", handlers.Profile(tmpls.Profile))

	// Admin routes
	r.Get("/admin/users", handlers.AdminUsers(tmpls.AdminUsers))
	r.Get("/admin/users/new", handlers.AdminCreateUser(tmpls.AdminUserForm))
	r.Post("/admin/users/new", handlers.AdminCreateUser(tmpls.AdminUserForm))
	r.Get("/admin/users/{id}/edit", handlers.AdminEditUser(tmpls.AdminUserForm))
	r.Post("/admin/users/{id}/edit", handlers.AdminEditUser(tmpls.AdminUserForm))
	r.Post("/admin/users/{id}/delete", handlers.AdminDeleteUser)
	r.Post("/admin/ingredients/import", handlers.ImportIngredients)

	return r
}

func doGet(t *testing.T, client *http.Client, targetURL string) (int, string) {
	t.Helper()

	ctx := t.Context()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed GET %s: %v", targetURL, err)
	}

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	return resp.StatusCode, string(bodyBytes)
}

func doPost(t *testing.T, client *http.Client, targetURL string, form url.Values) int {
	t.Helper()

	ctx := t.Context()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed POST %s: %v", targetURL, err)
	}

	defer resp.Body.Close()

	return resp.StatusCode
}

func TestDashboardLoads(t *testing.T) {
	client := testServer.Client()
	status, body := doGet(t, client, testServer.URL+"/")

	if status != http.StatusOK {
		t.Errorf("Dashboard: expected 200, got %d", status)
	}

	if !strings.Contains(body, "Today - MyCal") {
		t.Error("Dashboard: missing title")
	}

	if !strings.Contains(body, "kcal") {
		t.Error("Dashboard: missing calorie display")
	}
}

func TestIngredientsListLoads(t *testing.T) {
	client := testServer.Client()
	status, body := doGet(t, client, testServer.URL+"/ingredients")

	if status != http.StatusOK {
		t.Errorf("Ingredients: expected 200, got %d", status)
	}

	if !strings.Contains(body, "Ingredients - MyCal") {
		t.Error("Ingredients: missing title")
	}
}

func TestFoodsListLoads(t *testing.T) {
	client := testServer.Client()
	status, body := doGet(t, client, testServer.URL+"/foods")

	if status != http.StatusOK {
		t.Errorf("Foods: expected 200, got %d", status)
	}

	if !strings.Contains(body, "Foods - MyCal") {
		t.Error("Foods: missing title")
	}
}

func TestIngredientFormLoads(t *testing.T) {
	client := testServer.Client()
	status, body := doGet(t, client, testServer.URL+"/ingredients/new")

	if status != http.StatusOK {
		t.Errorf("Ingredient form: expected 200, got %d", status)
	}

	if !strings.Contains(body, "Add Ingredient - MyCal") {
		t.Error("Ingredient form: missing title")
	}

	if !strings.Contains(body, `<form`) {
		t.Error("Ingredient form: missing form element")
	}
}

func TestIngredientCreateAndVerify(t *testing.T) {
	client := testServer.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	form := url.Values{}
	form.Set("name", "Integration Test Oatmeal")
	form.Set("calories", "150")
	form.Set("protein", "5")
	form.Set("carbs", "27")
	form.Set("fat", "3")
	form.Set("serving_size", "100g")

	status := doPost(t, client, testServer.URL+"/ingredients/new", form)

	if status != http.StatusSeeOther {
		t.Errorf("Create ingredient: expected 303 redirect, got %d", status)
	}

	// Verify ingredient appears in list
	_, body := doGet(t, client, testServer.URL+"/ingredients")

	if !strings.Contains(body, "Integration Test Oatmeal") {
		t.Error("Ingredients list: created ingredient not found")
	}

	if !strings.Contains(body, "150 kcal") {
		t.Error("Ingredients list: calorie count not found")
	}
}

func TestStaticFilesServed(t *testing.T) {
	client := testServer.Client()
	status, body := doGet(t, client, testServer.URL+"/static/css/style.css")

	if status != http.StatusOK {
		t.Errorf("Static CSS: expected 200, got %d", status)
	}

	if !strings.Contains(body, "container-narrow") {
		t.Error("CSS file does not contain expected styles")
	}
}

func TestErrorNotFound(t *testing.T) {
	client := testServer.Client()
	status, _ := doGet(t, client, testServer.URL+"/ingredients/99999/edit")

	if status != http.StatusNotFound {
		t.Errorf("Non-existent ingredient: expected 404, got %d", status)
	}
}

func TestErrorInvalidID(t *testing.T) {
	client := testServer.Client()
	status, _ := doGet(t, client, testServer.URL+"/ingredients/invalid/edit")

	if status != http.StatusBadRequest {
		t.Errorf("Invalid ingredient ID: expected 400, got %d", status)
	}
}

func TestErrorInvalidFormData(t *testing.T) {
	client := testServer.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	form := url.Values{}
	form.Set("name", "Bad")
	form.Set("calories", "not-a-number")
	form.Set("protein", "5")
	form.Set("carbs", "10")
	form.Set("fat", "2")
	form.Set("serving_size", "100g")

	status := doPost(t, client, testServer.URL+"/ingredients/new", form)

	if status != http.StatusBadRequest {
		t.Errorf("Invalid calories: expected 400, got %d", status)
	}
}

func TestDateNavigation(t *testing.T) {
	client := testServer.Client()
	status, body := doGet(t, client, testServer.URL+"/?date=2024-06-15")

	if status != http.StatusOK {
		t.Errorf("Dashboard with date: expected 200, got %d", status)
	}

	// Should show prev/next day links
	if !strings.Contains(body, "2024-06-14") {
		t.Error("Dashboard: previous day link not found")
	}

	if !strings.Contains(body, "2024-06-16") {
		t.Error("Dashboard: next day link not found")
	}

	if !strings.Contains(body, "2024-06-15") {
		t.Error("Dashboard: current date not found")
	}
}

func TestProfileLoads(t *testing.T) {
	client := testServer.Client()
	status, body := doGet(t, client, testServer.URL+"/profile")

	if status != http.StatusOK {
		t.Errorf("Profile: expected 200, got %d", status)
	}

	if !strings.Contains(body, "Profile - MyCal") {
		t.Error("Profile: missing title")
	}

	if !strings.Contains(body, "Daily Calories Goal") {
		t.Error("Profile: missing calories goal field")
	}
}

func TestProfileUpdate(t *testing.T) {
	client := testServer.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	form := url.Values{}
	form.Set("calories_goal", "2500")
	form.Set("protein_goal", "180")
	form.Set("carbs_goal", "300")
	form.Set("fat_goal", "80")

	status := doPost(t, client, testServer.URL+"/profile", form)

	if status != http.StatusSeeOther {
		t.Errorf("Update profile: expected 303 redirect, got %d", status)
	}

	// Verify values were saved
	_, body := doGet(t, client, testServer.URL+"/profile")

	if !strings.Contains(body, "2500") {
		t.Error("Profile: updated calories goal not found")
	}
}

// ============================================================================
// Authentication Tests
// ============================================================================

// TestLoginPageLoads verifies the login page renders correctly
func TestLoginPageLoads(t *testing.T) {
	client := testServer.Client()
	status, body := doGet(t, client, testServer.URL+"/login")

	if status != http.StatusOK {
		t.Errorf("Login page: expected 200, got %d", status)
	}

	if !strings.Contains(body, "Login") {
		t.Error("Login page: missing title")
	}

	if !strings.Contains(body, `<form`) {
		t.Error("Login page: missing form element")
	}

	if !strings.Contains(body, "username") {
		t.Error("Login page: missing username field")
	}

	if !strings.Contains(body, "password") {
		t.Error("Login page: missing password field")
	}
}

// TestLoginWithValidCredentials verifies login works with correct credentials
func TestLoginWithValidCredentials(t *testing.T) {
	// Create a separate server without auto-injected user for this test
	tmpDir, err := os.MkdirTemp("", "mycal-login-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	defer os.RemoveAll(tmpDir)

	// We need to test against the main test database since it has our user
	client := testServer.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	// Create a test user for login testing
	loginUser, err := auth.CreateUser("logintest", "loginpass123", false)
	if err != nil {
		t.Fatalf("Failed to create login test user: %v", err)
	}

	_ = loginUser // Used for creating the user

	form := url.Values{}
	form.Set("username", "logintest")
	form.Set("password", "loginpass123")

	status := doPost(t, client, testServer.URL+"/login", form)

	// Login should redirect to dashboard on success
	if status != http.StatusSeeOther {
		t.Errorf("Login: expected 303 redirect, got %d", status)
	}
}

// TestLoginWithInvalidCredentials verifies login fails with wrong password
func TestLoginWithInvalidCredentials(t *testing.T) {
	client := testServer.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	form := url.Values{}
	form.Set("username", "testuser")
	form.Set("password", "wrongpassword")

	ctx := t.Context()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, testServer.URL+"/login", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed POST /login: %v", err)
	}

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	body := string(bodyBytes)

	// Should show login page with error, not redirect
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Invalid login: expected 200 with error, got %d", resp.StatusCode)
	}

	if !strings.Contains(body, "Invalid") || !strings.Contains(body, "password") {
		t.Error("Login page should show error message for invalid credentials")
	}
}

// TestLoginWithNonExistentUser verifies login fails for unknown user
func TestLoginWithNonExistentUser(t *testing.T) {
	client := testServer.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	form := url.Values{}
	form.Set("username", "nonexistentuser12345")
	form.Set("password", "anypassword")

	ctx := t.Context()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, testServer.URL+"/login", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed POST /login: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Non-existent user login: expected 200 with error, got %d", resp.StatusCode)
	}
}

// TestLogout verifies logout clears session
func TestLogout(t *testing.T) {
	client := testServer.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	status := doPost(t, client, testServer.URL+"/logout", url.Values{})

	// Logout should redirect to login page
	if status != http.StatusSeeOther {
		t.Errorf("Logout: expected 303 redirect, got %d", status)
	}
}

// ============================================================================
// Admin Route Tests
// ============================================================================

// TestAdminUsersPageLoads verifies admin users page works for admin
func TestAdminUsersPageLoads(t *testing.T) {
	client := testServer.Client()
	status, body := doGet(t, client, testServer.URL+"/admin/users")

	if status != http.StatusOK {
		t.Errorf("Admin users: expected 200, got %d", status)
	}

	if !strings.Contains(body, "Users") {
		t.Error("Admin users: missing title")
	}
}

// TestAdminCreateUserFormLoads verifies admin create user form loads
func TestAdminCreateUserFormLoads(t *testing.T) {
	client := testServer.Client()
	status, body := doGet(t, client, testServer.URL+"/admin/users/new")

	if status != http.StatusOK {
		t.Errorf("Admin create user: expected 200, got %d", status)
	}

	if !strings.Contains(body, "User") {
		t.Error("Admin create user: missing title")
	}

	if !strings.Contains(body, `<form`) {
		t.Error("Admin create user: missing form")
	}
}

// TestAdminCreateUser verifies admin can create new users
func TestAdminCreateUser(t *testing.T) {
	client := testServer.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	form := url.Values{}
	form.Set("username", "newadmincreated")
	form.Set("password", "newpassword123")
	form.Set("is_admin", "false")

	status := doPost(t, client, testServer.URL+"/admin/users/new", form)

	if status != http.StatusSeeOther {
		t.Errorf("Admin create user: expected 303 redirect, got %d", status)
	}

	// Verify user appears in list
	_, body := doGet(t, client, testServer.URL+"/admin/users")

	if !strings.Contains(body, "newadmincreated") {
		t.Error("Admin users: newly created user not found")
	}
}

// ============================================================================
// Route Protection Tests (Unauthenticated Access)
// ============================================================================

// setupUnauthenticatedServer creates a test server without auto-injected user
// to test route protection behavior
func setupUnauthenticatedServer(t *testing.T) *httptest.Server {
	t.Helper()

	tmpls, err := loadTestTemplates()
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	// Use real auth middleware (no injected user)
	r.Use(auth.RequireSetup)

	// Static files (no auth required)
	fs := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	// Public routes
	r.Get("/login", handlers.Login(tmpls.Login))
	r.Post("/login", handlers.Login(tmpls.Login))
	r.Get("/setup", handlers.Setup(tmpls.Setup))
	r.Post("/setup", handlers.Setup(tmpls.Setup))

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)

		r.Get("/", handlers.Dashboard(tmpls.Dashboard))
		r.Post("/logout", handlers.Logout)

		r.Get("/ingredients", handlers.ListIngredients(tmpls.Ingredients))
		r.Get("/ingredients/new", handlers.CreateIngredient(tmpls.IngredientForm))
		r.Post("/ingredients/new", handlers.CreateIngredient(tmpls.IngredientForm))
		r.Get("/ingredients/{id}/edit", handlers.EditIngredient(tmpls.IngredientForm))

		r.Get("/foods", handlers.ListFoods(tmpls.Foods))
		r.Get("/foods/new", handlers.CreateFood(tmpls.FoodForm))

		r.Get("/entries/{id}/edit", handlers.GetEntry(tmpls.EntryForm))

		r.Get("/profile", handlers.Profile(tmpls.Profile))
		r.Post("/profile", handlers.Profile(tmpls.Profile))

		// Admin routes
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAdmin)
			r.Get("/admin/users", handlers.AdminUsers(tmpls.AdminUsers))
			r.Get("/admin/users/new", handlers.AdminCreateUser(tmpls.AdminUserForm))
			r.Post("/admin/users/new", handlers.AdminCreateUser(tmpls.AdminUserForm))
			r.Post("/admin/ingredients/import", handlers.ImportIngredients)
		})
	})

	return httptest.NewServer(r)
}

// setupNonAdminServer creates a test server with a non-admin user to test admin protection
func setupNonAdminServer(t *testing.T) (*httptest.Server, *models.User) {
	t.Helper()

	// Create non-admin user
	nonAdminUser, err := auth.CreateUser("nonadmin_"+strconv.FormatInt(time.Now().UnixNano(), 36), "password123", false)
	if err != nil {
		t.Fatalf("Failed to create non-admin user: %v", err)
	}

	tmpls, err := loadTestTemplates()
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	// Inject non-admin user into context
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), auth.UserContextKey, nonAdminUser)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Get("/", handlers.Dashboard(tmpls.Dashboard))
		r.Get("/ingredients", handlers.ListIngredients(tmpls.Ingredients))
		r.Get("/foods", handlers.ListFoods(tmpls.Foods))
		r.Get("/profile", handlers.Profile(tmpls.Profile))

		// Admin routes
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAdmin)
			r.Get("/admin/users", handlers.AdminUsers(tmpls.AdminUsers))
			r.Get("/admin/users/new", handlers.AdminCreateUser(tmpls.AdminUserForm))
			r.Post("/admin/users/new", handlers.AdminCreateUser(tmpls.AdminUserForm))
			r.Post("/admin/ingredients/import", handlers.ImportIngredients)
		})
	})

	return httptest.NewServer(r), nonAdminUser
}

// TestProtectedRoutesRedirectToLogin verifies that protected routes redirect
// unauthenticated users to the login page
func TestProtectedRoutesRedirectToLogin(t *testing.T) {
	server := setupUnauthenticatedServer(t)
	defer server.Close()

	client := server.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	protectedRoutes := []string{
		"/",
		"/ingredients",
		"/ingredients/new",
		"/foods",
		"/foods/new",
		"/profile",
		"/admin/users",
		"/admin/users/new",
	}

	for _, route := range protectedRoutes {
		t.Run(route, func(t *testing.T) {
			ctx := t.Context()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+route, http.NoBody)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed GET %s: %v", route, err)
			}

			defer resp.Body.Close()

			// Should redirect to /login
			if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
				t.Errorf("Route %s: expected redirect (302/303), got %d", route, resp.StatusCode)

				return
			}

			location := resp.Header.Get("Location")
			if !strings.Contains(location, "/login") {
				t.Errorf("Route %s: expected redirect to /login, got %s", route, location)
			}
		})
	}
}

// TestPublicRoutesAccessible verifies that public routes are accessible without auth
func TestPublicRoutesAccessible(t *testing.T) {
	server := setupUnauthenticatedServer(t)
	defer server.Close()

	client := server.Client()

	publicRoutes := []string{
		"/login",
	}

	for _, route := range publicRoutes {
		t.Run(route, func(t *testing.T) {
			ctx := t.Context()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+route, http.NoBody)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed GET %s: %v", route, err)
			}

			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Route %s: expected 200, got %d", route, resp.StatusCode)
			}
		})
	}
}

// TestStaticFilesAccessibleWithoutAuth verifies static files don't require auth
func TestStaticFilesAccessibleWithoutAuth(t *testing.T) {
	server := setupUnauthenticatedServer(t)
	defer server.Close()

	client := server.Client()
	ctx := t.Context()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/static/css/style.css", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed GET /static/css/style.css: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Static files should be accessible without auth, got %d", resp.StatusCode)
	}
}

// ============================================================================
// Admin Permission Tests (Non-Admin Access)
// ============================================================================

// TestAdminRoutesRequireAdmin verifies admin routes return 403 for non-admin users
func TestAdminRoutesRequireAdmin(t *testing.T) {
	server, _ := setupNonAdminServer(t)
	defer server.Close()

	client := server.Client()

	adminRoutes := []string{
		"/admin/users",
		"/admin/users/new",
	}

	for _, route := range adminRoutes {
		t.Run(route, func(t *testing.T) {
			ctx := t.Context()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+route, http.NoBody)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed GET %s: %v", route, err)
			}

			defer resp.Body.Close()

			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("Route %s: expected 403 Forbidden for non-admin, got %d", route, resp.StatusCode)
			}
		})
	}
}

// TestAdminImportRouteRequiresAdmin verifies admin import route returns 403 for non-admin
func TestAdminImportRouteRequiresAdmin(t *testing.T) {
	server, _ := setupNonAdminServer(t)
	defer server.Close()

	client := server.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	ctx := t.Context()

	// POST to admin import endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/admin/ingredients/import", strings.NewReader(""))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "multipart/form-data")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed POST /admin/ingredients/import: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Admin import: expected 403 Forbidden for non-admin, got %d", resp.StatusCode)
	}
}

// TestNonAdminCanAccessRegularRoutes verifies non-admin users can access regular routes
func TestNonAdminCanAccessRegularRoutes(t *testing.T) {
	server, _ := setupNonAdminServer(t)
	defer server.Close()

	client := server.Client()

	regularRoutes := []string{
		"/",
		"/ingredients",
		"/foods",
		"/profile",
	}

	for _, route := range regularRoutes {
		t.Run(route, func(t *testing.T) {
			ctx := t.Context()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+route, http.NoBody)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed GET %s: %v", route, err)
			}

			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Route %s: non-admin should access regular route, got %d", route, resp.StatusCode)
			}
		})
	}
}
