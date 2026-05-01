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
	}

	base := filepath.Join("templates", "base.html")

	dashboard, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join("templates", "dashboard.html"))
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

	return &Templates{
		Dashboard: dashboard,
		Foods:     foods,
		FoodForm:  foodForm,
		EntryForm: entryForm,
	}, nil
}

func setupRouter(tmpls *Templates) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	fs := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	r.Get("/", handlers.Dashboard(tmpls.Dashboard))
	r.Get("/foods", handlers.ListFoods(tmpls.Foods))
	r.Get("/foods/new", handlers.CreateFood(tmpls.FoodForm))
	r.Post("/foods/new", handlers.CreateFood(tmpls.FoodForm))
	r.Get("/foods/{id}/edit", handlers.EditFood(tmpls.FoodForm))
	r.Post("/foods/{id}/edit", handlers.EditFood(tmpls.FoodForm))
	r.Post("/foods/{id}/delete", handlers.DeleteFood)
	r.Get("/foods/search", handlers.SearchFoods)
	r.Post("/entries", handlers.CreateEntry)
	r.Get("/entries/{id}/edit", handlers.GetEntry(tmpls.EntryForm))
	r.Post("/entries/{id}/edit", handlers.UpdateEntry)
	r.Post("/entries/{id}/delete", handlers.DeleteEntry)

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

func TestFoodFormLoads(t *testing.T) {
	client := testServer.Client()
	status, body := doGet(t, client, testServer.URL+"/foods/new")

	if status != http.StatusOK {
		t.Errorf("Food form: expected 200, got %d", status)
	}

	if !strings.Contains(body, "Add Food - MyCal") {
		t.Error("Food form: missing title")
	}

	if !strings.Contains(body, `<form`) {
		t.Error("Food form: missing form element")
	}
}

func TestFoodCreateAndVerify(t *testing.T) {
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
	form.Set("serving_size", "1 cup cooked")

	status := doPost(t, client, testServer.URL+"/foods/new", form)

	if status != http.StatusSeeOther {
		t.Errorf("Create food: expected 303 redirect, got %d", status)
	}

	// Verify food appears in list
	_, body := doGet(t, client, testServer.URL+"/foods")

	if !strings.Contains(body, "Integration Test Oatmeal") {
		t.Error("Foods list: created food not found")
	}

	if !strings.Contains(body, "150 kcal") {
		t.Error("Foods list: calorie count not found")
	}
}

func TestEntryCreateAndDisplay(t *testing.T) {
	client := testServer.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	form := url.Values{}
	form.Set("food_id", "1")
	form.Set("date", time.Now().Format("2006-01-02"))
	form.Set("meal", "breakfast")
	form.Set("servings", "2")

	status := doPost(t, client, testServer.URL+"/entries", form)

	if status != http.StatusSeeOther {
		t.Errorf("Create entry: expected 303 redirect, got %d", status)
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
	status, _ := doGet(t, client, testServer.URL+"/foods/99999/edit")

	if status != http.StatusNotFound {
		t.Errorf("Non-existent food: expected 404, got %d", status)
	}
}

func TestErrorInvalidID(t *testing.T) {
	client := testServer.Client()
	status, _ := doGet(t, client, testServer.URL+"/foods/invalid/edit")

	if status != http.StatusBadRequest {
		t.Errorf("Invalid food ID: expected 400, got %d", status)
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
	form.Set("serving_size", "1")

	status := doPost(t, client, testServer.URL+"/foods/new", form)

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
