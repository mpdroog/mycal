package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mpdroog/mycal/auth"
	"github.com/mpdroog/mycal/db"
	"github.com/mpdroog/mycal/handlers"
)

const compressionLevel = 5

// Version is set at build time via ldflags
// go build -ldflags "-X main.Version=$(git rev-parse --short HEAD)"
var Version = "dev"

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
			return weekday // "Tuesday" (this week, future)
		}

		if days >= -6 && days <= -2 {
			return "Last " + weekday // "Last Tuesday"
		}

		if days > 6 && days <= 13 {
			return "Next " + weekday // "Next Tuesday"
		}

		// For dates further away, show weekday with relative weeks
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
	"subtract": func(a, b int) int {
		return a - b
	},
	"version": func() string {
		return Version
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
	Login          *template.Template
	Setup          *template.Template
	AdminUsers     *template.Template
	AdminUserForm  *template.Template
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

	// Static files (no auth required)
	// Strip version from paths like style.vdev.css → style.css for cache busting
	versionPattern := regexp.MustCompile(`\.v[^.]+\.`)
	fs := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip /static/ prefix and version suffix
		path := r.URL.Path[len("/static/"):]
		path = versionPattern.ReplaceAllString(path, ".")
		r.URL.Path = "/" + path
		fs.ServeHTTP(w, r)
	}))

	// Public routes (no auth required)
	r.Get("/login", handlers.Login(tmpls.Login))
	r.Post("/login", handlers.Login(tmpls.Login))
	r.Get("/setup", handlers.Setup(tmpls.Setup))
	r.Post("/setup", handlers.Setup(tmpls.Setup))

	// Protected routes (require auth)
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireSetup)
		r.Use(auth.RequireAuth)

		// Dashboard
		r.Get("/", handlers.Dashboard(tmpls.Dashboard))
		r.Post("/logout", handlers.Logout)

		// Ingredients (base nutritional items) - shared across all users
		r.Get("/ingredients", handlers.ListIngredients(tmpls.Ingredients))
		r.Get("/ingredients/new", handlers.CreateIngredient(tmpls.IngredientForm))
		r.Post("/ingredients/new", handlers.CreateIngredient(tmpls.IngredientForm))
		r.Get("/ingredients/{id}/edit", handlers.EditIngredient(tmpls.IngredientForm))
		r.Post("/ingredients/{id}/edit", handlers.EditIngredient(tmpls.IngredientForm))
		r.Post("/ingredients/{id}/delete", handlers.DeleteIngredient)
		r.Post("/ingredients/{id}/restore", handlers.RestoreIngredient)
		r.Get("/ingredients/search", handlers.SearchIngredients)

		// Foods (combinations of ingredients) - shared across all users
		r.Get("/foods", handlers.ListFoods(tmpls.Foods))
		r.Get("/foods/new", handlers.CreateFood(tmpls.FoodForm))
		r.Post("/foods/new", handlers.CreateFood(tmpls.FoodForm))
		r.Get("/foods/{id}/edit", handlers.EditFood(tmpls.FoodForm))
		r.Post("/foods/{id}/edit", handlers.EditFood(tmpls.FoodForm))
		r.Post("/foods/{id}/delete", handlers.DeleteFood)
		r.Post("/foods/{id}/restore", handlers.RestoreFood)
		r.Get("/foods/search", handlers.SearchFoods)

		// Entries - per-user
		r.Post("/entries", handlers.CreateEntry)
		r.Get("/entries/{id}/edit", handlers.GetEntry(tmpls.EntryForm))
		r.Post("/entries/{id}/edit", handlers.UpdateEntry)
		r.Post("/entries/{id}/servings", handlers.UpdateEntryServings)
		r.Post("/entries/{id}/delete", handlers.DeleteEntry)
		r.Post("/entries/{id}/restore", handlers.RestoreEntry)

		// Profile - per-user
		r.Get("/profile", handlers.Profile(tmpls.Profile))
		r.Post("/profile", handlers.Profile(tmpls.Profile))

		// Admin routes
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAdmin)
			r.Get("/admin/users", handlers.AdminUsers(tmpls.AdminUsers))
			r.Get("/admin/users/new", handlers.AdminCreateUser(tmpls.AdminUserForm))
			r.Post("/admin/users/new", handlers.AdminCreateUser(tmpls.AdminUserForm))
			r.Get("/admin/users/{id}/edit", handlers.AdminEditUser(tmpls.AdminUserForm))
			r.Post("/admin/users/{id}/edit", handlers.AdminEditUser(tmpls.AdminUserForm))
			r.Post("/admin/users/{id}/delete", handlers.AdminDeleteUser)
			r.Post("/admin/ingredients/import", handlers.ImportIngredients)
		})
	})

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
