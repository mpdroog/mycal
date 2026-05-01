package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mpdroog/mycal/db"
	"github.com/mpdroog/mycal/handlers"
)

const compressionLevel = 5

var funcMap = template.FuncMap{
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

// Templates holds all parsed page templates.
type Templates struct {
	Dashboard      *template.Template
	Ingredients    *template.Template
	IngredientForm *template.Template
	Foods          *template.Template
	FoodForm       *template.Template
	EntryForm      *template.Template
	Profile        *template.Template
}

func loadTemplates() (*Templates, error) {
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

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dataDir := flag.String("data", "./data", "Data directory for SQLite database")
	flag.Parse()

	// Initialize database
	if err := db.Init(*dataDir); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	// Parse templates
	tmpls, err := loadTemplates()
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// Setup router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(compressionLevel))

	// Static files
	fs := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	// Routes
	r.Get("/", handlers.Dashboard(tmpls.Dashboard))

	// Ingredients (base nutritional items)
	r.Get("/ingredients", handlers.ListIngredients(tmpls.Ingredients))
	r.Get("/ingredients/new", handlers.CreateIngredient(tmpls.IngredientForm))
	r.Post("/ingredients/new", handlers.CreateIngredient(tmpls.IngredientForm))
	r.Get("/ingredients/{id}/edit", handlers.EditIngredient(tmpls.IngredientForm))
	r.Post("/ingredients/{id}/edit", handlers.EditIngredient(tmpls.IngredientForm))
	r.Post("/ingredients/{id}/delete", handlers.DeleteIngredient)
	r.Get("/ingredients/search", handlers.SearchIngredients)

	// Foods (combinations of ingredients)
	r.Get("/foods", handlers.ListFoods(tmpls.Foods))
	r.Get("/foods/new", handlers.CreateFood(tmpls.FoodForm))
	r.Post("/foods/new", handlers.CreateFood(tmpls.FoodForm))
	r.Get("/foods/{id}/edit", handlers.EditFood(tmpls.FoodForm))
	r.Post("/foods/{id}/edit", handlers.EditFood(tmpls.FoodForm))
	r.Post("/foods/{id}/delete", handlers.DeleteFood)
	r.Get("/foods/search", handlers.SearchFoods)

	// Entries
	r.Post("/entries", handlers.CreateEntry)
	r.Get("/entries/{id}/edit", handlers.GetEntry(tmpls.EntryForm))
	r.Post("/entries/{id}/edit", handlers.UpdateEntry)
	r.Post("/entries/{id}/delete", handlers.DeleteEntry)

	// Profile
	r.Get("/profile", handlers.Profile(tmpls.Profile))
	r.Post("/profile", handlers.Profile(tmpls.Profile))

	log.Printf("Starting MyCal on %s", *addr)

	server := &http.Server{
		Addr:              *addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
