package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mpdroog/mycal/auth"
	"github.com/mpdroog/mycal/db"
	"github.com/mpdroog/mycal/handlers"
	"github.com/mpdroog/mycal/tmpl"
)

const compressionLevel = 5

// Version is set at build time via ldflags
// go build -ldflags "-X main.Version=$(git rev-parse --short HEAD)"
var Version = "dev"

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dataDir := flag.String("data", "./data", "Data directory for SQLite database")
	verbose := flag.Bool("v", false, "Verbose logging")
	insecure := flag.Bool("insecure", false, "Disable secure cookies (for local development without HTTPS)")
	flag.Parse()

	if *insecure {
		auth.SecureCookie = false
	}

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
	tmpls, err := tmpl.Load("templates", Version)
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// Setup router
	r := chi.NewRouter()
	if *verbose {
		r.Use(middleware.Logger)
	}
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(compressionLevel))
	r.Use(auth.CheckCSRF)

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

		// Unified fuzzy search for foods and ingredients
		r.Get("/search", handlers.Search)

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

	if *verbose {
		log.Printf("MyCal %s", Version)
		log.Printf("  addr=%s data=%s", *addr, *dataDir)
	}

	// Create listener first so we can notify systemd when ready
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	server := &http.Server{
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Notify systemd we're ready (no-op if not running under systemd)
	sent, err := daemon.SdNotify(false, daemon.SdNotifyReady)
	if err != nil {
		log.Printf("sd_notify failed: %v", err)
	} else if !sent {
		log.Printf("sd_notify: NOTIFY_SOCKET not set")
	}

	if err := server.Serve(listener); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
