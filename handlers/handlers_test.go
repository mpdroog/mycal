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
	Dashboard      *template.Template
	Ingredients    *template.Template
	IngredientForm *template.Template
	Foods          *template.Template
	FoodForm       *template.Template
	EntryForm      *template.Template
	Profile        *template.Template
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

	// Find templates directory (relative to test location)
	templatesDir := filepath.Join("..", "templates")
	base := filepath.Join(templatesDir, "base.html")

	dashboard, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join(templatesDir, "dashboard.html"))
	if err != nil {
		return nil, err
	}

	ingredients, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join(templatesDir, "ingredients.html"))
	if err != nil {
		return nil, err
	}

	ingredientForm, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join(templatesDir, "ingredient_form.html"))
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

	profile, err := template.New("").Funcs(funcMap).ParseFiles(base, filepath.Join(templatesDir, "profile.html"))
	if err != nil {
		return nil, err
	}

	return &TestTemplates{
		Dashboard:      dashboard,
		Ingredients:    ingredients,
		IngredientForm: ingredientForm,
		Foods:          foods,
		FoodForm:       foodForm,
		EntryForm:      entryForm,
		Profile:        profile,
	}, nil
}

func setupTestRouter() *chi.Mux {
	r := chi.NewRouter()

	r.Get("/", handlers.Dashboard(testTemplates.Dashboard))

	// Ingredients
	r.Get("/ingredients", handlers.ListIngredients(testTemplates.Ingredients))
	r.Get("/ingredients/new", handlers.CreateIngredient(testTemplates.IngredientForm))
	r.Post("/ingredients/new", handlers.CreateIngredient(testTemplates.IngredientForm))
	r.Get("/ingredients/{id}/edit", handlers.EditIngredient(testTemplates.IngredientForm))
	r.Post("/ingredients/{id}/edit", handlers.EditIngredient(testTemplates.IngredientForm))
	r.Post("/ingredients/{id}/delete", handlers.DeleteIngredient)
	r.Get("/ingredients/search", handlers.SearchIngredients)

	// Foods
	r.Get("/foods", handlers.ListFoods(testTemplates.Foods))
	r.Get("/foods/new", handlers.CreateFood(testTemplates.FoodForm))
	r.Post("/foods/new", handlers.CreateFood(testTemplates.FoodForm))
	r.Get("/foods/{id}/edit", handlers.EditFood(testTemplates.FoodForm))
	r.Post("/foods/{id}/edit", handlers.EditFood(testTemplates.FoodForm))
	r.Post("/foods/{id}/delete", handlers.DeleteFood)

	// Entries
	r.Post("/entries", handlers.CreateEntry)
	r.Get("/entries/{id}/edit", handlers.GetEntry(testTemplates.EntryForm))
	r.Post("/entries/{id}/edit", handlers.UpdateEntry)
	r.Post("/entries/{id}/servings", handlers.UpdateEntryServings)
	r.Post("/entries/{id}/delete", handlers.DeleteEntry)

	// Profile
	r.Get("/profile", handlers.Profile(testTemplates.Profile))
	r.Post("/profile", handlers.Profile(testTemplates.Profile))

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

	if !strings.Contains(body, "kcal") {
		t.Error("Dashboard: expected 'kcal' in response body")
	}
}

func TestIngredientsList(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ingredients", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Ingredients list: expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, "Ingredients") {
		t.Error("Ingredients list: expected 'Ingredients' in response body")
	}
}

func TestIngredientFormGet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ingredients/new", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Ingredient form GET: expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, "Add Ingredient") {
		t.Error("Ingredient form GET: expected 'Add Ingredient' in response body")
	}

	if !strings.Contains(body, "<form") {
		t.Error("Ingredient form GET: expected form element in response body")
	}

	if !strings.Contains(body, "calories") {
		t.Error("Ingredient form GET: expected 'calories' field in response body")
	}
}

func TestIngredientCreateAndList(t *testing.T) {
	// Create an ingredient
	form := url.Values{}
	form.Set("name", "Test Chicken")
	form.Set("calories", "165")
	form.Set("protein", "31")
	form.Set("carbs", "0")
	form.Set("fat", "3.6")
	form.Set("serving_size", "100g")

	req := httptest.NewRequest(http.MethodPost, "/ingredients/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("Ingredient create: expected redirect (303), got %d", rec.Code)
	}

	// Verify ingredient appears in list
	req = httptest.NewRequest(http.MethodGet, "/ingredients", http.NoBody)
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	body := rec.Body.String()

	if !strings.Contains(body, "Test Chicken") {
		t.Error("Ingredient list: expected 'Test Chicken' in response after creation")
	}

	if !strings.Contains(body, "165 kcal") {
		t.Error("Ingredient list: expected '165 kcal' in response")
	}
}

func TestIngredientSearch(t *testing.T) {
	// First create an ingredient to search for
	form := url.Values{}
	form.Set("name", "Searchable Apple")
	form.Set("calories", "95")
	form.Set("protein", "0.5")
	form.Set("carbs", "25")
	form.Set("fat", "0.3")
	form.Set("serving_size", "1 medium")

	req := httptest.NewRequest(http.MethodPost, "/ingredients/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	// Now search
	req = httptest.NewRequest(http.MethodGet, "/ingredients/search?q=Apple", http.NoBody)
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Ingredient search: expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, "Searchable Apple") {
		t.Error("Ingredient search: expected 'Searchable Apple' in search results")
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

	if !strings.Contains(body, "Food Name") {
		t.Error("Food form GET: expected 'Food Name' field in response body")
	}

	if !strings.Contains(body, "Ingredients") {
		t.Error("Food form GET: expected 'Ingredients' section in response body")
	}
}

func TestFoodCreateAndList(t *testing.T) {
	// Create a food (without ingredients for simplicity)
	form := url.Values{}
	form.Set("name", "Test Breakfast Bowl")
	form.Set("ingredients", "[]")

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

	if !strings.Contains(body, "Test Breakfast Bowl") {
		t.Error("Food list: expected 'Test Breakfast Bowl' in response after creation")
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

func TestIngredientEditNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ingredients/99999/edit", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Ingredient edit not found: expected status 404, got %d", rec.Code)
	}
}

func TestInvalidIngredientId(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ingredients/invalid/edit", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Invalid ingredient id: expected status 400, got %d", rec.Code)
	}
}

func TestIngredientFormValidation(t *testing.T) {
	// Test with invalid calories
	form := url.Values{}
	form.Set("name", "Bad Ingredient")
	form.Set("calories", "not-a-number")
	form.Set("protein", "10")
	form.Set("carbs", "20")
	form.Set("fat", "5")
	form.Set("serving_size", "100g")

	req := httptest.NewRequest(http.MethodPost, "/ingredients/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Invalid calories: expected status 400, got %d", rec.Code)
	}
}

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
		{"Protein display", "Protein"},
		{"Carbs display", "Carbs"},
		{"Fat display", "Fat"},
		{"Calorie unit", "kcal"},
		{"Add Entry form", `action="/entries"`},
		{"Date picker", `type="date"`},
	}

	for _, check := range checks {
		if !strings.Contains(body, check.content) {
			t.Errorf("Dashboard missing %s: expected %q in body", check.name, check.content)
		}
	}
}

func TestIngredientsPageHTMLStructure(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ingredients", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	body := rec.Body.String()

	checks := []struct {
		name    string
		content string
	}{
		{"DOCTYPE", "<!DOCTYPE html>"},
		{"title tag", "<title>Ingredients - MyCal</title>"},
		{"navbar", `class="navbar`},
		{"Ingredients nav active", `href="/ingredients" class="nav-link active">Ingredients</a>`},
		{"Add Ingredient button", `href="/ingredients/new"`},
		{"Ingredients heading", ">Ingredients</h5>"},
	}

	for _, check := range checks {
		if !strings.Contains(body, check.content) {
			t.Errorf("Ingredients page missing %s: expected %q in body", check.name, check.content)
		}
	}
}

func TestIngredientFormHTMLStructure(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ingredients/new", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	body := rec.Body.String()

	checks := []struct {
		name    string
		content string
	}{
		{"DOCTYPE", "<!DOCTYPE html>"},
		{"title tag", "<title>Add Ingredient - MyCal</title>"},
		{"form tag", "<form"},
		{"POST method", `method="POST"`},
		{"name field", `name="name"`},
		{"calories field", `name="calories"`},
		{"protein field", `name="protein"`},
		{"carbs field", `name="carbs"`},
		{"fat field", `name="fat"`},
		{"serving_size field", `name="serving_size"`},
		{"submit button", `type="submit"`},
		{"cancel link", `href="/ingredients"`},
	}

	for _, check := range checks {
		if !strings.Contains(body, check.content) {
			t.Errorf("Ingredient form missing %s: expected %q in body", check.name, check.content)
		}
	}
}

func TestIngredientEditShowsCorrectData(t *testing.T) {
	// First create an ingredient
	form := url.Values{}
	form.Set("name", "Edit Test Banana")
	form.Set("calories", "105")
	form.Set("protein", "1.3")
	form.Set("carbs", "27")
	form.Set("fat", "0.4")
	form.Set("serving_size", "1 medium")

	req := httptest.NewRequest(http.MethodPost, "/ingredients/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	// Get the ingredient ID
	var ingredientID int64

	err := db.DB.QueryRow("SELECT id FROM ingredients WHERE name = ?", "Edit Test Banana").Scan(&ingredientID)
	if err != nil {
		t.Fatalf("Could not find created ingredient: %v", err)
	}

	// Now fetch the edit form
	req = httptest.NewRequest(http.MethodGet, "/ingredients/"+strconv.FormatInt(ingredientID, 10)+"/edit", http.NoBody)
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Ingredient edit: expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	checks := []struct {
		name    string
		content string
	}{
		{"title", "<title>Edit Ingredient - MyCal</title>"},
		{"ingredient name value", `value="Edit Test Banana"`},
		{"calories value", `value="105"`},
	}

	for _, check := range checks {
		if !strings.Contains(body, check.content) {
			t.Errorf("Ingredient edit form missing %s: expected %q in body", check.name, check.content)
		}
	}
}

func TestIngredientDeleteRemovesFromList(t *testing.T) {
	// First create an ingredient
	form := url.Values{}
	form.Set("name", "Delete Test Ingredient")
	form.Set("calories", "100")
	form.Set("protein", "5")
	form.Set("carbs", "10")
	form.Set("fat", "2")
	form.Set("serving_size", "100g")

	req := httptest.NewRequest(http.MethodPost, "/ingredients/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	// Get the ingredient ID
	var ingredientID int64

	err := db.DB.QueryRow("SELECT id FROM ingredients WHERE name = ?", "Delete Test Ingredient").Scan(&ingredientID)
	if err != nil {
		t.Fatalf("Could not find created ingredient: %v", err)
	}

	// Verify it appears in the list
	req = httptest.NewRequest(http.MethodGet, "/ingredients", http.NoBody)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "Delete Test Ingredient") {
		t.Error("Ingredient should appear in list before deletion")
	}

	// Delete the ingredient
	req = httptest.NewRequest(http.MethodPost, "/ingredients/"+strconv.FormatInt(ingredientID, 10)+"/delete", http.NoBody)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("Ingredient delete: expected redirect (303), got %d", rec.Code)
	}

	// Verify it no longer appears in the list
	req = httptest.NewRequest(http.MethodGet, "/ingredients", http.NoBody)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if strings.Contains(rec.Body.String(), "Delete Test Ingredient") {
		t.Error("Ingredient should not appear in list after deletion")
	}
}

func TestProfileGet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/profile", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Profile GET: expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, "Profile") {
		t.Error("Profile page: expected 'Profile' in response body")
	}

	if !strings.Contains(body, "Daily Calories Goal") {
		t.Error("Profile page: expected 'Daily Calories Goal' in response body")
	}

	if !strings.Contains(body, "Protein") {
		t.Error("Profile page: expected 'Protein' field in response body")
	}
}

func TestProfileUpdate(t *testing.T) {
	form := url.Values{}
	form.Set("calories_goal", "2200")
	form.Set("protein_goal", "160")
	form.Set("carbs_goal", "280")
	form.Set("fat_goal", "70")

	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("Profile update: expected redirect (303), got %d", rec.Code)
	}

	// Verify values were saved
	req = httptest.NewRequest(http.MethodGet, "/profile", http.NoBody)
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	body := rec.Body.String()

	if !strings.Contains(body, "2200") {
		t.Error("Profile: updated calories goal not found")
	}
}

func TestDashboardShowsProfileGoals(t *testing.T) {
	// First update profile with specific values
	form := url.Values{}
	form.Set("calories_goal", "1800")
	form.Set("protein_goal", "120")
	form.Set("carbs_goal", "200")
	form.Set("fat_goal", "60")

	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	// Now check dashboard shows these goals
	req = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	body := rec.Body.String()

	if !strings.Contains(body, "1800") {
		t.Error("Dashboard: expected calorie goal '1800' from profile")
	}

	if !strings.Contains(body, "/ 120g") {
		t.Error("Dashboard: expected protein goal '120g' from profile")
	}

	if !strings.Contains(body, "/ 200g") {
		t.Error("Dashboard: expected carbs goal '200g' from profile")
	}

	if !strings.Contains(body, "/ 60g") {
		t.Error("Dashboard: expected fat goal '60g' from profile")
	}
}
