package main

import (
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mpdroog/mycal/db"
	"github.com/mpdroog/mycal/handlers"
)

var testServer *httptest.Server

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

	// Setup templates and server
	tmpls, err := loadTestTemplates()
	if err != nil {
		panic(err)
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

	return &Templates{
		Dashboard:      dashboard,
		Ingredients:    ingredients,
		IngredientForm: ingredientForm,
		Foods:          foods,
		FoodForm:       foodForm,
		EntryForm:      entryForm,
		Profile:        profile,
	}, nil
}

func setupRouter(tmpls *Templates) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	fs := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	r.Get("/", handlers.Dashboard(tmpls.Dashboard))

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
	r.Post("/entries/{id}/delete", handlers.DeleteEntry)

	// Profile
	r.Get("/profile", handlers.Profile(tmpls.Profile))
	r.Post("/profile", handlers.Profile(tmpls.Profile))

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
