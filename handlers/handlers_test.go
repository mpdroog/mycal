package handlers_test

import (
	"html/template"
	"log"
	"mime/multipart"
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
		"relativeDate": func(date string) string {
			t, parseErr := time.Parse("2006-01-02", date)
			if parseErr != nil {
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
	r.Post("/ingredients/{id}/restore", handlers.RestoreIngredient)
	r.Get("/ingredients/search", handlers.SearchIngredients)
	r.Post("/ingredients/import", handlers.ImportIngredients)

	// Foods
	r.Get("/foods", handlers.ListFoods(testTemplates.Foods))
	r.Get("/foods/new", handlers.CreateFood(testTemplates.FoodForm))
	r.Post("/foods/new", handlers.CreateFood(testTemplates.FoodForm))
	r.Get("/foods/{id}/edit", handlers.EditFood(testTemplates.FoodForm))
	r.Post("/foods/{id}/edit", handlers.EditFood(testTemplates.FoodForm))
	r.Post("/foods/{id}/delete", handlers.DeleteFood)
	r.Post("/foods/{id}/restore", handlers.RestoreFood)

	// Entries
	r.Post("/entries", handlers.CreateEntry)
	r.Get("/entries/{id}/edit", handlers.GetEntry(testTemplates.EntryForm))
	r.Post("/entries/{id}/edit", handlers.UpdateEntry)
	r.Post("/entries/{id}/servings", handlers.UpdateEntryServings)
	r.Post("/entries/{id}/delete", handlers.DeleteEntry)
	r.Post("/entries/{id}/restore", handlers.RestoreEntry)

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

func TestDeleteEntryDoesNotAffectOtherEntries(t *testing.T) {
	// Create an ingredient
	form := url.Values{}
	form.Set("name", "Test Entry Deletion Ingredient")
	form.Set("calories", "100")
	form.Set("protein", "10")
	form.Set("carbs", "10")
	form.Set("fat", "5")
	form.Set("serving_size", "100g")

	req := httptest.NewRequest(http.MethodPost, "/ingredients/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	// Create a food with this ingredient
	var ingredientID int64

	err := db.DB.QueryRow("SELECT id FROM ingredients WHERE name = ?", "Test Entry Deletion Ingredient").Scan(&ingredientID)
	if err != nil {
		t.Fatalf("Could not find ingredient: %v", err)
	}

	form = url.Values{}
	form.Set("name", "Test Entry Deletion Food")
	form.Set("ingredients", `[{"ingredient_id": `+strconv.FormatInt(ingredientID, 10)+`, "amount_grams": 100}]`)

	req = httptest.NewRequest(http.MethodPost, "/foods/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	// Get the food ID
	var foodID int64

	err = db.DB.QueryRow("SELECT id FROM foods WHERE name = ?", "Test Entry Deletion Food").Scan(&foodID)
	if err != nil {
		t.Fatalf("Could not find food: %v", err)
	}

	// Create two entries with this food
	_, err = db.DB.Exec(`INSERT INTO entries (food_id, date, meal, servings) VALUES (?, '2024-01-01', 'breakfast', 1)`, foodID)
	if err != nil {
		t.Fatalf("Could not create entry 1: %v", err)
	}

	_, err = db.DB.Exec(`INSERT INTO entries (food_id, date, meal, servings) VALUES (?, '2024-01-01', 'lunch', 1)`, foodID)
	if err != nil {
		t.Fatalf("Could not create entry 2: %v", err)
	}

	// Get entry IDs
	var entry1ID, entry2ID int64

	rows, err := db.DB.Query("SELECT id FROM entries WHERE food_id = ? ORDER BY id", foodID)
	if err != nil {
		t.Fatalf("Could not query entries: %v", err)
	}

	if rows.Next() {
		rows.Scan(&entry1ID)
	}

	if rows.Next() {
		rows.Scan(&entry2ID)
	}

	rows.Close() // Close immediately before doing more DB operations

	if entry1ID == 0 || entry2ID == 0 {
		t.Fatal("Could not get entry IDs")
	}

	// Delete the first entry
	req = httptest.NewRequest(http.MethodPost, "/entries/"+strconv.FormatInt(entry1ID, 10)+"/delete", http.NoBody)
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("Delete entry: expected 303, got %d", rec.Code)
	}

	// Verify second entry still exists
	var count int

	err = db.DB.QueryRow("SELECT COUNT(*) FROM entries WHERE id = ?", entry2ID).Scan(&count)
	if err != nil {
		t.Fatalf("Could not check entry 2: %v", err)
	}

	if count != 1 {
		t.Error("Deleting one entry should not delete other entries")
	}

	// Verify the food still exists
	err = db.DB.QueryRow("SELECT COUNT(*) FROM foods WHERE id = ?", foodID).Scan(&count)
	if err != nil {
		t.Fatalf("Could not check food: %v", err)
	}

	if count != 1 {
		t.Error("Deleting an entry should not delete the food")
	}

	// Verify the ingredient still exists
	err = db.DB.QueryRow("SELECT COUNT(*) FROM ingredients WHERE id = ?", ingredientID).Scan(&count)
	if err != nil {
		t.Fatalf("Could not check ingredient: %v", err)
	}

	if count != 1 {
		t.Error("Deleting an entry should not delete the ingredient")
	}
}

func TestCannotDeleteFoodWithEntries(t *testing.T) {
	// Create an ingredient
	form := url.Values{}
	form.Set("name", "Protected Food Ingredient")
	form.Set("calories", "100")
	form.Set("protein", "10")
	form.Set("carbs", "10")
	form.Set("fat", "5")
	form.Set("serving_size", "100g")

	req := httptest.NewRequest(http.MethodPost, "/ingredients/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var ingredientID int64

	err := db.DB.QueryRow("SELECT id FROM ingredients WHERE name = ?", "Protected Food Ingredient").Scan(&ingredientID)
	if err != nil {
		t.Fatalf("Could not find ingredient: %v", err)
	}

	// Create a food
	form = url.Values{}
	form.Set("name", "Protected Food")
	form.Set("ingredients", `[{"ingredient_id": `+strconv.FormatInt(ingredientID, 10)+`, "amount_grams": 100}]`)

	req = httptest.NewRequest(http.MethodPost, "/foods/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var foodID int64

	err = db.DB.QueryRow("SELECT id FROM foods WHERE name = ?", "Protected Food").Scan(&foodID)
	if err != nil {
		t.Fatalf("Could not find food: %v", err)
	}

	// Create an entry using this food
	_, err = db.DB.Exec(`INSERT INTO entries (food_id, date, meal, servings) VALUES (?, '2024-01-02', 'dinner', 1)`, foodID)
	if err != nil {
		t.Fatalf("Could not create entry: %v", err)
	}

	// Try to delete the food - should fail
	req = httptest.NewRequest(http.MethodPost, "/foods/"+strconv.FormatInt(foodID, 10)+"/delete", http.NoBody)
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("Delete food with entries: expected 409 Conflict, got %d", rec.Code)
	}

	// Verify food still exists
	var count int

	err = db.DB.QueryRow("SELECT COUNT(*) FROM foods WHERE id = ?", foodID).Scan(&count)
	if err != nil {
		t.Fatalf("Could not check food: %v", err)
	}

	if count != 1 {
		t.Error("Food should not be deleted when it has entries")
	}
}

func TestCannotDeleteIngredientUsedInFoods(t *testing.T) {
	// Create an ingredient
	form := url.Values{}
	form.Set("name", "Protected Ingredient")
	form.Set("calories", "100")
	form.Set("protein", "10")
	form.Set("carbs", "10")
	form.Set("fat", "5")
	form.Set("serving_size", "100g")

	req := httptest.NewRequest(http.MethodPost, "/ingredients/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var ingredientID int64

	err := db.DB.QueryRow("SELECT id FROM ingredients WHERE name = ?", "Protected Ingredient").Scan(&ingredientID)
	if err != nil {
		t.Fatalf("Could not find ingredient: %v", err)
	}

	// Create a food using this ingredient
	form = url.Values{}
	form.Set("name", "Food Using Protected Ingredient")
	form.Set("ingredients", `[{"ingredient_id": `+strconv.FormatInt(ingredientID, 10)+`, "amount_grams": 100}]`)

	req = httptest.NewRequest(http.MethodPost, "/foods/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	// Try to delete the ingredient - should fail
	req = httptest.NewRequest(http.MethodPost, "/ingredients/"+strconv.FormatInt(ingredientID, 10)+"/delete", http.NoBody)
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("Delete ingredient used in food: expected 409 Conflict, got %d", rec.Code)
	}

	// Verify ingredient still exists
	var count int

	err = db.DB.QueryRow("SELECT COUNT(*) FROM ingredients WHERE id = ?", ingredientID).Scan(&count)
	if err != nil {
		t.Fatalf("Could not check ingredient: %v", err)
	}

	if count != 1 {
		t.Error("Ingredient should not be deleted when used in foods")
	}
}

func TestFloatingSummaryPresentWithoutEntries(t *testing.T) {
	// Get dashboard for a date with no entries
	req := httptest.NewRequest(http.MethodGet, "/?date=1999-01-01", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Dashboard: expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	// Verify floating summary HTML is present
	if !strings.Contains(body, `id="floatingSummary"`) {
		t.Error("Floating summary HTML element should be present even without entries")
	}

	// Verify the dashboard JavaScript is loaded (scroll handler logic is in external file)
	if !strings.Contains(body, `src="/static/js/dashboard.js"`) {
		t.Error("Dashboard JavaScript should be loaded")
	}

	// Verify dashboard config element is present for JS to read
	if !strings.Contains(body, `id="dashboardConfig"`) {
		t.Error("Dashboard config element should be present for JavaScript")
	}
}

func TestEntryServingsPersistedAndDisplayed(t *testing.T) {
	// Create an ingredient with known calories (100 kcal per 100g)
	form := url.Values{}
	form.Set("name", "Test Servings Ingredient")
	form.Set("calories", "100")
	form.Set("protein", "10")
	form.Set("carbs", "10")
	form.Set("fat", "5")
	form.Set("serving_size", "100g")

	req := httptest.NewRequest(http.MethodPost, "/ingredients/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var ingredientID int64

	err := db.DB.QueryRow("SELECT id FROM ingredients WHERE name = ?", "Test Servings Ingredient").Scan(&ingredientID)
	if err != nil {
		t.Fatalf("Could not find ingredient: %v", err)
	}

	// Create a food with this ingredient
	form = url.Values{}
	form.Set("name", "Test Servings Food")
	form.Set("ingredients", `[{"ingredient_id": `+strconv.FormatInt(ingredientID, 10)+`, "amount_grams": 100}]`)

	req = httptest.NewRequest(http.MethodPost, "/foods/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var foodID int64

	err = db.DB.QueryRow("SELECT id FROM foods WHERE name = ?", "Test Servings Food").Scan(&foodID)
	if err != nil {
		t.Fatalf("Could not find food: %v", err)
	}

	// Create an entry with servings = 1
	testDate := "2024-03-15"
	form = url.Values{}
	form.Set("food_id", strconv.FormatInt(foodID, 10))
	form.Set("date", testDate)
	form.Set("meal", "lunch")
	form.Set("servings", "1")

	req = httptest.NewRequest(http.MethodPost, "/entries", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var entryID int64

	err = db.DB.QueryRow("SELECT id FROM entries WHERE food_id = ? AND date = ?", foodID, testDate).Scan(&entryID)
	if err != nil {
		t.Fatalf("Could not find entry: %v", err)
	}

	// Update servings to 5
	form = url.Values{}
	form.Set("servings", "5")

	req = httptest.NewRequest(http.MethodPost, "/entries/"+strconv.FormatInt(entryID, 10)+"/servings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Update servings: expected 200, got %d", rec.Code)
	}

	// Verify servings was saved in database
	var savedServings float64

	err = db.DB.QueryRow("SELECT servings FROM entries WHERE id = ?", entryID).Scan(&savedServings)
	if err != nil {
		t.Fatalf("Could not query entry: %v", err)
	}

	if savedServings != 5.0 {
		t.Errorf("Servings not saved correctly: expected 5.0, got %.2f", savedServings)
	}

	// Load the dashboard and verify it shows 500 kcal (100 kcal * 5 servings)
	req = httptest.NewRequest(http.MethodGet, "/?date="+testDate, http.NoBody)
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Dashboard: expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	// The entry row should show 500 kcal
	if !strings.Contains(body, "500 kcal") {
		// Find what kcal value is actually shown
		idx := strings.Index(body, "entry-calories")
		if idx != -1 {
			snippet := body[idx : idx+100]
			t.Errorf("Dashboard should show 500 kcal (100 kcal * 5 servings) after reload. Found: %s", snippet)
		} else {
			t.Error("Dashboard should show 500 kcal (100 kcal * 5 servings) after reload - entry-calories not found")
		}
	}

	// The servings input should show 5
	if !strings.Contains(body, `value="5"`) && !strings.Contains(body, `value="5.0"`) {
		t.Error("Servings input should show value 5 after reload")
	}

	// The total in the header should also be 500
	// Look for the eaten calories display
	if !strings.Contains(body, ">500<") {
		t.Error("Dashboard header should show 500 calories eaten")
	}
}

func TestQuickAddIngredientServingsPersistedAndDisplayed(t *testing.T) {
	// Create an ingredient with known calories (200 kcal per 100g)
	form := url.Values{}
	form.Set("name", "Quick Add Test Ingredient")
	form.Set("calories", "200")
	form.Set("protein", "20")
	form.Set("carbs", "10")
	form.Set("fat", "10")
	form.Set("serving_size", "100g")

	req := httptest.NewRequest(http.MethodPost, "/ingredients/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var ingredientID int64

	err := db.DB.QueryRow("SELECT id FROM ingredients WHERE name = ?", "Quick Add Test Ingredient").Scan(&ingredientID)
	if err != nil {
		t.Fatalf("Could not find ingredient: %v", err)
	}

	// Quick add: Create entry directly from ingredient (this auto-creates a food)
	testDate := "2024-03-16"
	form = url.Values{}
	form.Set("ingredient_id", strconv.FormatInt(ingredientID, 10))
	form.Set("date", testDate)
	form.Set("meal", "dinner")
	form.Set("servings", "1")

	req = httptest.NewRequest(http.MethodPost, "/entries", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("Create entry: expected 303, got %d", rec.Code)
	}

	// Find the entry
	var entryID int64

	err = db.DB.QueryRow("SELECT id FROM entries WHERE date = ?", testDate).Scan(&entryID)
	if err != nil {
		t.Fatalf("Could not find entry: %v", err)
	}

	// Update servings to 5
	form = url.Values{}
	form.Set("servings", "5")

	req = httptest.NewRequest(http.MethodPost, "/entries/"+strconv.FormatInt(entryID, 10)+"/servings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Update servings: expected 200, got %d", rec.Code)
	}

	// Verify servings was saved in database
	var savedServings float64

	err = db.DB.QueryRow("SELECT servings FROM entries WHERE id = ?", entryID).Scan(&savedServings)
	if err != nil {
		t.Fatalf("Could not query entry: %v", err)
	}

	if savedServings != 5.0 {
		t.Errorf("Servings not saved correctly: expected 5.0, got %.2f", savedServings)
	}

	// Load the dashboard and verify it shows 1000 kcal (200 kcal * 5 servings)
	req = httptest.NewRequest(http.MethodGet, "/?date="+testDate, http.NoBody)
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Dashboard: expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	// The entry row should show 1000 kcal (200 * 5)
	if !strings.Contains(body, "1000 kcal") {
		// Find what kcal value is actually shown
		idx := strings.Index(body, "entry-calories")
		if idx != -1 {
			snippet := body[idx : idx+100]
			t.Errorf("Dashboard should show 1000 kcal (200 kcal * 5 servings) after reload. Found: %s", snippet)
		} else {
			t.Error("Dashboard should show 1000 kcal (200 kcal * 5 servings) after reload - entry-calories not found")
		}
	}

	// The servings input should show 5
	if !strings.Contains(body, `value="5"`) && !strings.Contains(body, `value="5.0"`) {
		t.Error("Servings input should show value 5 after reload")
	}
}

func TestServingsUpdateWithMultipartForm(t *testing.T) {
	// This test uses multipart/form-data like the JavaScript FormData does

	// Create test data
	_, err := db.DB.Exec(`INSERT INTO ingredients (name, calories, protein, carbs, fat, serving_size) VALUES ('Multipart Test Ing', 150, 15, 15, 5, '100g')`)
	if err != nil {
		t.Fatalf("Create ingredient: %v", err)
	}

	var ingID int64

	db.DB.QueryRow("SELECT id FROM ingredients WHERE name = 'Multipart Test Ing'").Scan(&ingID)

	_, err = db.DB.Exec(`INSERT INTO foods (name) VALUES ('Multipart Test Food')`)
	if err != nil {
		t.Fatalf("Create food: %v", err)
	}

	var foodID int64

	db.DB.QueryRow("SELECT id FROM foods WHERE name = 'Multipart Test Food'").Scan(&foodID)

	_, err = db.DB.Exec(`INSERT INTO food_ingredients (food_id, ingredient_id, amount_grams) VALUES (?, ?, 100)`, foodID, ingID)
	if err != nil {
		t.Fatalf("Link ingredient: %v", err)
	}

	_, err = db.DB.Exec(`INSERT INTO entries (food_id, date, meal, servings) VALUES (?, '2024-03-17', 'lunch', 1)`, foodID)
	if err != nil {
		t.Fatalf("Create entry: %v", err)
	}

	var entryID int64

	db.DB.QueryRow("SELECT id FROM entries WHERE food_id = ? AND date = '2024-03-17'", foodID).Scan(&entryID)

	// Create multipart form data (like JavaScript FormData)
	body := &strings.Builder{}
	writer := multipart.NewWriter(body)
	writer.WriteField("servings", "5")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/entries/"+strconv.FormatInt(entryID, 10)+"/servings", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Update servings with multipart: expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}

	// Verify servings was saved
	var savedServings float64

	err = db.DB.QueryRow("SELECT servings FROM entries WHERE id = ?", entryID).Scan(&savedServings)
	if err != nil {
		t.Fatalf("Query entry: %v", err)
	}

	if savedServings != 5.0 {
		t.Errorf("Servings not saved with multipart form: expected 5.0, got %.2f", savedServings)
	}
}

func TestEntryUndoRestore(t *testing.T) {
	// Create test ingredient
	_, err := db.DB.Exec(`INSERT INTO ingredients (name, calories, protein, carbs, fat, serving_size) VALUES ('Undo Test Ing', 100, 10, 10, 5, '100g')`)
	if err != nil {
		t.Fatalf("Create ingredient: %v", err)
	}

	var ingID int64

	db.DB.QueryRow("SELECT id FROM ingredients WHERE name = 'Undo Test Ing'").Scan(&ingID)

	// Create test food
	_, err = db.DB.Exec(`INSERT INTO foods (name) VALUES ('Undo Test Food')`)
	if err != nil {
		t.Fatalf("Create food: %v", err)
	}

	var foodID int64

	db.DB.QueryRow("SELECT id FROM foods WHERE name = 'Undo Test Food'").Scan(&foodID)

	_, err = db.DB.Exec(`INSERT INTO food_ingredients (food_id, ingredient_id, amount_grams) VALUES (?, ?, 100)`, foodID, ingID)
	if err != nil {
		t.Fatalf("Link ingredient: %v", err)
	}

	// Create entry
	_, err = db.DB.Exec(`INSERT INTO entries (food_id, date, meal, servings) VALUES (?, '2024-05-01', 'breakfast', 1)`, foodID)
	if err != nil {
		t.Fatalf("Create entry: %v", err)
	}

	var entryID int64

	db.DB.QueryRow("SELECT id FROM entries WHERE food_id = ? AND date = '2024-05-01'", foodID).Scan(&entryID)

	// Delete the entry
	req := httptest.NewRequest(http.MethodPost, "/entries/"+strconv.FormatInt(entryID, 10)+"/delete", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("Delete entry: expected 303, got %d", rec.Code)
	}

	// Verify redirect URL has correct date format and undo params
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "date=2024-05-01") {
		t.Errorf("Delete redirect should contain date=2024-05-01, got: %s", location)
	}

	if !strings.Contains(location, "deleted=entry") {
		t.Errorf("Delete redirect should contain deleted=entry, got: %s", location)
	}

	if !strings.Contains(location, "id="+strconv.FormatInt(entryID, 10)) {
		t.Errorf("Delete redirect should contain id=%d, got: %s", entryID, location)
	}

	// Verify entry is soft-deleted (deleted_at is set)
	var deletedAt *string

	err = db.DB.QueryRow("SELECT deleted_at FROM entries WHERE id = ?", entryID).Scan(&deletedAt)
	if err != nil {
		t.Fatalf("Query entry: %v", err)
	}

	if deletedAt == nil {
		t.Error("Entry should have deleted_at set after delete")
	}

	// Verify entry doesn't appear in dashboard query (filtered by deleted_at IS NULL)
	var visibleCount int

	err = db.DB.QueryRow("SELECT COUNT(*) FROM entries WHERE id = ? AND deleted_at IS NULL", entryID).Scan(&visibleCount)
	if err != nil {
		t.Fatalf("Count visible entries: %v", err)
	}

	if visibleCount != 0 {
		t.Error("Deleted entry should not be visible (deleted_at should filter it out)")
	}

	// Restore the entry (undo)
	req = httptest.NewRequest(http.MethodPost, "/entries/"+strconv.FormatInt(entryID, 10)+"/restore", http.NoBody)
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("Restore entry: expected 303, got %d", rec.Code)
	}

	// Verify entry is restored (deleted_at is NULL)
	err = db.DB.QueryRow("SELECT deleted_at FROM entries WHERE id = ?", entryID).Scan(&deletedAt)
	if err != nil {
		t.Fatalf("Query entry after restore: %v", err)
	}

	if deletedAt != nil {
		t.Error("Entry should have deleted_at = NULL after restore")
	}

	// Verify entry appears again
	err = db.DB.QueryRow("SELECT COUNT(*) FROM entries WHERE id = ? AND deleted_at IS NULL", entryID).Scan(&visibleCount)
	if err != nil {
		t.Fatalf("Count visible entries after restore: %v", err)
	}

	if visibleCount != 1 {
		t.Error("Restored entry should be visible again")
	}
}

func TestDeleteEntryDateFormatNormalization(t *testing.T) {
	// This test verifies that even if the database stores dates in timestamp format,
	// the redirect URL will have a properly formatted date (YYYY-MM-DD)

	// Create test data with a date that might be stored as timestamp
	_, err := db.DB.Exec(`INSERT INTO ingredients (name, calories, protein, carbs, fat, serving_size) VALUES ('Date Format Test Ing', 100, 10, 10, 5, '100g')`)
	if err != nil {
		t.Fatalf("Create ingredient: %v", err)
	}

	var ingID int64

	db.DB.QueryRow("SELECT id FROM ingredients WHERE name = 'Date Format Test Ing'").Scan(&ingID)

	_, err = db.DB.Exec(`INSERT INTO foods (name) VALUES ('Date Format Test Food')`)
	if err != nil {
		t.Fatalf("Create food: %v", err)
	}

	var foodID int64

	db.DB.QueryRow("SELECT id FROM foods WHERE name = 'Date Format Test Food'").Scan(&foodID)

	_, err = db.DB.Exec(`INSERT INTO food_ingredients (food_id, ingredient_id, amount_grams) VALUES (?, ?, 100)`, foodID, ingID)
	if err != nil {
		t.Fatalf("Link ingredient: %v", err)
	}

	// Create entry with a specific date
	_, err = db.DB.Exec(`INSERT INTO entries (food_id, date, meal, servings) VALUES (?, '2024-06-15', 'lunch', 1)`, foodID)
	if err != nil {
		t.Fatalf("Create entry: %v", err)
	}

	var entryID int64

	db.DB.QueryRow("SELECT id FROM entries WHERE food_id = ? AND meal = 'lunch'", foodID).Scan(&entryID)

	// Delete the entry
	req := httptest.NewRequest(http.MethodPost, "/entries/"+strconv.FormatInt(entryID, 10)+"/delete", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	// Check that the redirect URL has a properly formatted date
	location := rec.Header().Get("Location")

	// The date should be in YYYY-MM-DD format, not contain 'T' (timestamp format)
	if strings.Contains(location, "T") {
		t.Errorf("Redirect URL should not contain timestamp format, got: %s", location)
	}

	// Should contain the proper date format
	if !strings.Contains(location, "date=2024-06-15") {
		t.Errorf("Redirect URL should contain date=2024-06-15, got: %s", location)
	}
}

func TestDashboardDateNormalization(t *testing.T) {
	// Test that the dashboard normalizes timestamp-format dates in URL

	// Request dashboard with a timestamp-format date
	req := httptest.NewRequest(http.MethodGet, "/?date=2024-07-20T00:00:00Z", http.NoBody)
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Dashboard with timestamp date: expected 200, got %d", rec.Code)
	}

	// The response body should show the normalized date (not the timestamp)
	body := rec.Body.String()

	// Should show the proper date format in the page, not the timestamp
	if strings.Contains(body, "T00:00:00Z") {
		t.Error("Dashboard should normalize timestamp dates, but found timestamp in response")
	}

	// Should contain the normalized date
	if !strings.Contains(body, "2024-07-20") {
		t.Error("Dashboard should display normalized date 2024-07-20")
	}
}
