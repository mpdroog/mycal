package handlers_test

import (
	"html/template"
	"log"
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

	"github.com/mpdroog/mycal/db"
	"github.com/mpdroog/mycal/handlers"
)

var (
	testTemplates *TestTemplates
	testRouter    *chi.Mux
)

type TestTemplates struct {
	Dashboard *template.Template
	Foods     *template.Template
	FoodForm  *template.Template
	EntryForm *template.Template
}

func TestMain(m *testing.M) {
	// Create temp directory for test database
	tmpDir, err := os.MkdirTemp("", "mycal-test-*")
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
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Error closing test database: %v", closeErr)
		}
	}()

	// Load templates
	testTemplates, err = loadTestTemplates()
	if err != nil {
		panic(err)
	}

	// Setup router
	testRouter = setupTestRouter()

	os.Exit(m.Run())
}

func loadTestTemplates() (*TestTemplates, error) {
	funcMap := template.FuncMap{
		"prevDay": func(date string) string {
			t, parseErr := time.Parse("2006-01-02", date)
			if parseErr != nil {
				return date
			}

			return t.AddDate(0, 0, -1).Format("2006-01-02")
		},
		"nextDay": func(date string) string {
			t, parseErr := time.Parse("2006-01-02", date)
			if parseErr != nil {
				return date
			}

			return t.AddDate(0, 0, 1).Format("2006-01-02")
		},
		"title": cases.Title(language.English).String,
		"multiply": func(a int, b float64) int {
			return int(float64(a) * b)
		},
	}

	// Find templates directory (relative to test location)
	templatesDir := filepath.Join("..", "templates")
	base := filepath.Join(templatesDir, "base.html")

	dashboard, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join(templatesDir, "dashboard.html"))
	if err != nil {
		return nil, err
	}

	foods, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join(templatesDir, "foods.html"))
	if err != nil {
		return nil, err
	}

	foodForm, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join(templatesDir, "food_form.html"))
	if err != nil {
		return nil, err
	}

	entryForm, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join(templatesDir, "entry_form.html"))
	if err != nil {
		return nil, err
	}

	return &TestTemplates{
		Dashboard: dashboard,
		Foods:     foods,
		FoodForm:  foodForm,
		EntryForm: entryForm,
	}, nil
}

func setupTestRouter() *chi.Mux {
	r := chi.NewRouter()

	r.Get("/", handlers.Dashboard(testTemplates.Dashboard))
	r.Get("/foods", handlers.ListFoods(testTemplates.Foods))
	r.Get("/foods/new", handlers.CreateFood(testTemplates.FoodForm))
	r.Post("/foods/new", handlers.CreateFood(testTemplates.FoodForm))
	r.Get("/foods/{id}/edit", handlers.EditFood(testTemplates.FoodForm))
	r.Post("/foods/{id}/edit", handlers.EditFood(testTemplates.FoodForm))
	r.Post("/foods/{id}/delete", handlers.DeleteFood)
	r.Get("/foods/search", handlers.SearchFoods)
	r.Post("/entries", handlers.CreateEntry)
	r.Get("/entries/{id}/edit", handlers.GetEntry(testTemplates.EntryForm))
	r.Post("/entries/{id}/edit", handlers.UpdateEntry)
	r.Post("/entries/{id}/delete", handlers.DeleteEntry)

	return r
}

func TestDashboard(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Dashboard: expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, "Today") {
		t.Error("Dashboard: expected 'Today' in response body")
	}

	if !strings.Contains(body, "Daily Total") {
		t.Error("Dashboard: expected 'Daily Total' in response body")
	}
}

func TestFoodsList(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/foods", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Foods list: expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, "Foods") {
		t.Error("Foods list: expected 'Foods' in response body")
	}
}

func TestFoodFormGet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/foods/new", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Food form GET: expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, "Add Food") {
		t.Error("Food form GET: expected 'Add Food' in response body")
	}

	if !strings.Contains(body, "<form") {
		t.Error("Food form GET: expected form element in response body")
	}

	if !strings.Contains(body, "calories") {
		t.Error("Food form GET: expected 'calories' field in response body")
	}
}

func TestFoodCreateAndList(t *testing.T) {
	// Create a food
	form := url.Values{}
	form.Set("name", "Test Chicken")
	form.Set("calories", "165")
	form.Set("protein", "31")
	form.Set("carbs", "0")
	form.Set("fat", "3.6")
	form.Set("serving_size", "100g")

	req := httptest.NewRequest(http.MethodPost, "/foods/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("Food create: expected redirect (303), got %d", rec.Code)
	}

	// Verify food appears in list
	req = httptest.NewRequest(http.MethodGet, "/foods", http.NoBody)
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	body := rec.Body.String()

	if !strings.Contains(body, "Test Chicken") {
		t.Error("Food list: expected 'Test Chicken' in response after creation")
	}

	if !strings.Contains(body, "165 cal") {
		t.Error("Food list: expected '165 cal' in response")
	}
}

func TestFoodSearch(t *testing.T) {
	// First create a food to search for
	form := url.Values{}
	form.Set("name", "Searchable Apple")
	form.Set("calories", "95")
	form.Set("protein", "0.5")
	form.Set("carbs", "25")
	form.Set("fat", "0.3")
	form.Set("serving_size", "1 medium")

	req := httptest.NewRequest(http.MethodPost, "/foods/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	// Now search
	req = httptest.NewRequest(http.MethodGet, "/foods/search?q=Apple", http.NoBody)
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Food search: expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, "Searchable Apple") {
		t.Error("Food search: expected 'Searchable Apple' in search results")
	}
}

func TestEntryCreateAndDisplay(t *testing.T) {
	// First create a food
	form := url.Values{}
	form.Set("name", "Entry Test Food")
	form.Set("calories", "200")
	form.Set("protein", "10")
	form.Set("carbs", "20")
	form.Set("fat", "8")
	form.Set("serving_size", "1 portion")

	req := httptest.NewRequest(http.MethodPost, "/foods/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	// Get the food ID from the database
	var foodID int64

	err := db.DB.QueryRow("SELECT id FROM foods WHERE name = ?", "Entry Test Food").Scan(&foodID)
	if err != nil {
		t.Fatalf("Could not find created food: %v", err)
	}

	// Create an entry for today
	today := time.Now().Format("2006-01-02")
	form = url.Values{}
	form.Set("food_id", "1")
	form.Set("date", today)
	form.Set("meal", "lunch")
	form.Set("servings", "1.5")

	req = httptest.NewRequest(http.MethodPost, "/entries", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("Entry create: expected redirect (303), got %d. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestDashboardWithDate(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?date=2024-01-15", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Dashboard with date: expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, "2024-01-15") {
		t.Error("Dashboard: expected date '2024-01-15' in response")
	}
}

func TestFoodEditNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/foods/99999/edit", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Food edit not found: expected status 404, got %d", rec.Code)
	}
}

func TestInvalidFoodId(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/foods/invalid/edit", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Invalid food id: expected status 400, got %d", rec.Code)
	}
}

func TestFoodFormValidation(t *testing.T) {
	// Test with invalid calories
	form := url.Values{}
	form.Set("name", "Bad Food")
	form.Set("calories", "not-a-number")
	form.Set("protein", "10")
	form.Set("carbs", "20")
	form.Set("fat", "5")
	form.Set("serving_size", "1 cup")

	req := httptest.NewRequest(http.MethodPost, "/foods/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Invalid calories: expected status 400, got %d", rec.Code)
	}
}

// Integration tests to verify HTML structure

func TestDashboardHTMLStructure(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	body := rec.Body.String()

	// Check essential HTML structure
	checks := []struct {
		name    string
		content string
	}{
		{"DOCTYPE", "<!DOCTYPE html>"},
		{"title tag", "<title>Today - MyCal</title>"},
		{"navbar", `class="navbar`},
		{"Today nav link", `href="/" class="nav-link active">Today</a>`},
		{"Foods nav link", `href="/foods"`},
		{"Daily Total section", "Daily Total"},
		{"Calories display", "Calories"},
		{"Protein display", "Protein"},
		{"Carbs display", "Carbs"},
		{"Fat display", "Fat"},
		{"Add Entry form", `action="/entries"`},
		{"Date picker", `type="date"`},
	}

	for _, check := range checks {
		if !strings.Contains(body, check.content) {
			t.Errorf("Dashboard missing %s: expected %q in body", check.name, check.content)
		}
	}
}

func TestFoodsPageHTMLStructure(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/foods", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	body := rec.Body.String()

	checks := []struct {
		name    string
		content string
	}{
		{"DOCTYPE", "<!DOCTYPE html>"},
		{"title tag", "<title>Foods - MyCal</title>"},
		{"navbar", `class="navbar`},
		{"Foods nav active", `href="/foods" class="nav-link active">Foods</a>`},
		{"Add Food button", `href="/foods/new"`},
		{"Foods heading", ">Foods</h5>"},
	}

	for _, check := range checks {
		if !strings.Contains(body, check.content) {
			t.Errorf("Foods page missing %s: expected %q in body", check.name, check.content)
		}
	}
}

func TestFoodFormHTMLStructure(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/foods/new", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	body := rec.Body.String()

	checks := []struct {
		name    string
		content string
	}{
		{"DOCTYPE", "<!DOCTYPE html>"},
		{"title tag", "<title>Add Food - MyCal</title>"},
		{"form tag", "<form"},
		{"POST method", `method="POST"`},
		{"name field", `name="name"`},
		{"calories field", `name="calories"`},
		{"protein field", `name="protein"`},
		{"carbs field", `name="carbs"`},
		{"fat field", `name="fat"`},
		{"serving_size field", `name="serving_size"`},
		{"submit button", `type="submit"`},
		{"cancel link", `href="/foods"`},
	}

	for _, check := range checks {
		if !strings.Contains(body, check.content) {
			t.Errorf("Food form missing %s: expected %q in body", check.name, check.content)
		}
	}
}

func TestFoodEditShowsCorrectData(t *testing.T) {
	// First create a food
	form := url.Values{}
	form.Set("name", "Edit Test Banana")
	form.Set("calories", "105")
	form.Set("protein", "1.3")
	form.Set("carbs", "27")
	form.Set("fat", "0.4")
	form.Set("serving_size", "1 medium")

	req := httptest.NewRequest(http.MethodPost, "/foods/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	// Get the food ID
	var foodID int64

	err := db.DB.QueryRow("SELECT id FROM foods WHERE name = ?", "Edit Test Banana").Scan(&foodID)
	if err != nil {
		t.Fatalf("Could not find created food: %v", err)
	}

	// Now fetch the edit form
	req = httptest.NewRequest(http.MethodGet, "/foods/"+strconv.FormatInt(foodID, 10)+"/edit", http.NoBody)
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Food edit: expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	checks := []struct {
		name    string
		content string
	}{
		{"title", "<title>Edit Food - MyCal</title>"},
		{"food name value", `value="Edit Test Banana"`},
		{"calories value", `value="105"`},
	}

	for _, check := range checks {
		if !strings.Contains(body, check.content) {
			t.Errorf("Food edit form missing %s: expected %q in body", check.name, check.content)
		}
	}
}

func TestFoodDeleteRemovesFromList(t *testing.T) {
	// First create a food
	form := url.Values{}
	form.Set("name", "Delete Test Food")
	form.Set("calories", "100")
	form.Set("protein", "5")
	form.Set("carbs", "10")
	form.Set("fat", "2")
	form.Set("serving_size", "1 serving")

	req := httptest.NewRequest(http.MethodPost, "/foods/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	// Get the food ID
	var foodID int64

	err := db.DB.QueryRow("SELECT id FROM foods WHERE name = ?", "Delete Test Food").Scan(&foodID)
	if err != nil {
		t.Fatalf("Could not find created food: %v", err)
	}

	// Verify it appears in the list
	req = httptest.NewRequest(http.MethodGet, "/foods", http.NoBody)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "Delete Test Food") {
		t.Error("Food should appear in list before deletion")
	}

	// Delete the food
	req = httptest.NewRequest(http.MethodPost, "/foods/"+strconv.FormatInt(foodID, 10)+"/delete", http.NoBody)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("Food delete: expected redirect (303), got %d", rec.Code)
	}

	// Verify it no longer appears in the list
	req = httptest.NewRequest(http.MethodGet, "/foods", http.NoBody)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if strings.Contains(rec.Body.String(), "Delete Test Food") {
		t.Error("Food should not appear in list after deletion")
	}
}
