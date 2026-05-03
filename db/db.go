package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/mpdroog/mycal/models"
)

var DB *sql.DB

func Init(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "mycal.db")

	var err error

	DB, err = sql.Open("sqlite", dbPath+"?_txlock=immediate")
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	// Limit connections to avoid SQLite locking issues
	DB.SetMaxOpenConns(1)

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	// Set pragmas
	if _, err := DB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("set foreign_keys: %w", err)
	}

	if _, err := DB.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		return fmt.Errorf("set busy_timeout: %w", err)
	}

	// Share DB with models package
	models.DB = DB

	if err := migrate(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// Index existing items for fuzzy search (only if not already indexed)
	var trigramCount int

	if err := DB.QueryRow("SELECT COUNT(*) FROM search_trigrams").Scan(&trigramCount); err != nil {
		log.Printf("db: count trigrams: %v", err)
	}

	if trigramCount == 0 {
		if err := models.IndexAllItems(); err != nil {
			return fmt.Errorf("index items: %w", err)
		}
	}

	return nil
}

func Close() error {
	if DB != nil {
		return DB.Close()
	}

	return nil
}

func migrate() error {
	migrations := []string{
		// Search trigrams table for fuzzy search
		`CREATE TABLE IF NOT EXISTS search_trigrams (
			trigram TEXT NOT NULL,
			item_type TEXT NOT NULL,
			item_id INTEGER NOT NULL,
			PRIMARY KEY (trigram, item_type, item_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_trigrams_lookup ON search_trigrams(trigram)`,
		// Ingredients table (base nutritional items)
		`CREATE TABLE IF NOT EXISTS ingredients (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			calories INTEGER NOT NULL DEFAULT 0,
			protein REAL NOT NULL DEFAULT 0,
			carbs REAL NOT NULL DEFAULT 0,
			fat REAL NOT NULL DEFAULT 0,
			serving_size TEXT NOT NULL DEFAULT '100g',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// Foods table (combinations of ingredients)
		`CREATE TABLE IF NOT EXISTS foods (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// Junction table linking foods to ingredients with amounts
		`CREATE TABLE IF NOT EXISTS food_ingredients (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			food_id INTEGER NOT NULL REFERENCES foods(id) ON DELETE CASCADE,
			ingredient_id INTEGER NOT NULL REFERENCES ingredients(id) ON DELETE CASCADE,
			amount_grams REAL NOT NULL DEFAULT 100
		)`,
		`CREATE INDEX IF NOT EXISTS idx_food_ingredients_food ON food_ingredients(food_id)`,
		// Entries reference foods (not ingredients directly)
		`CREATE TABLE IF NOT EXISTS entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			food_id INTEGER REFERENCES foods(id) ON DELETE CASCADE,
			date DATE NOT NULL,
			meal TEXT NOT NULL DEFAULT 'snack',
			servings REAL NOT NULL DEFAULT 1,
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_entries_date ON entries(date)`,
		`CREATE INDEX IF NOT EXISTS idx_food_ingredients_ingredient ON food_ingredients(ingredient_id)`,
		`CREATE INDEX IF NOT EXISTS idx_entries_food ON entries(food_id)`,
		`CREATE TABLE IF NOT EXISTS meal_templates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			foods TEXT NOT NULL DEFAULT '[]'
		)`,
		`CREATE TABLE IF NOT EXISTS planned_meals (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date DATE NOT NULL,
			meal TEXT NOT NULL,
			template_id INTEGER REFERENCES meal_templates(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_planned_meals_date ON planned_meals(date)`,
		// Users table for authentication
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			is_admin INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// Sessions table for login state
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at)`,
	}

	for _, m := range migrations {
		if _, err := DB.Exec(m); err != nil {
			return fmt.Errorf("execute migration: %w", err)
		}
	}

	// Safe migrations that may fail if already applied (e.g., ADD COLUMN)
	safeMigrations := []string{
		`ALTER TABLE entries ADD COLUMN deleted_at DATETIME DEFAULT NULL`,
		`ALTER TABLE ingredients ADD COLUMN deleted_at DATETIME DEFAULT NULL`,
		`ALTER TABLE foods ADD COLUMN deleted_at DATETIME DEFAULT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_foods_deleted_at ON foods(deleted_at)`,
		`CREATE INDEX IF NOT EXISTS idx_ingredients_deleted_at ON ingredients(deleted_at)`,
		`CREATE INDEX IF NOT EXISTS idx_entries_deleted_at ON entries(deleted_at)`,
		`ALTER TABLE entries ADD COLUMN user_id INTEGER REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_entries_user ON entries(user_id)`,
		`ALTER TABLE ingredients ADD COLUMN serving_type TEXT NOT NULL DEFAULT 'weight'`,
		`ALTER TABLE foods ADD COLUMN serving_type TEXT NOT NULL DEFAULT 'weight'`,
		`ALTER TABLE foods ADD COLUMN serving_size TEXT NOT NULL DEFAULT '100g'`,
	}

	for _, m := range safeMigrations {
		// Errors expected if column already exists - log for debugging
		if _, err := DB.Exec(m); err != nil {
			log.Printf("db: safe migration (may be ok): %v", err)
		}
	}

	// Migrate profile table to support multiple users
	// Check if profile table has user_id column
	var hasUserID int

	err := DB.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('profile') WHERE name = 'user_id'`).Scan(&hasUserID)

	if err == nil && hasUserID == 0 {
		// Need to migrate profile table - create new structure
		if _, err := DB.Exec(`CREATE TABLE IF NOT EXISTS profile_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER UNIQUE REFERENCES users(id) ON DELETE CASCADE,
			calories_goal INTEGER NOT NULL DEFAULT 2000,
			protein_goal REAL NOT NULL DEFAULT 150,
			carbs_goal REAL NOT NULL DEFAULT 250,
			fat_goal REAL NOT NULL DEFAULT 65
		)`); err != nil {
			log.Printf("db: create profile_new: %v", err)
		}

		// Copy existing profile data (will be assigned to first user later)
		if _, err := DB.Exec(`INSERT INTO profile_new (calories_goal, protein_goal, carbs_goal, fat_goal)
			SELECT calories_goal, protein_goal, carbs_goal, fat_goal FROM profile WHERE id = 1`); err != nil {
			log.Printf("db: copy profile data: %v", err)
		}

		// Drop old table and rename
		if _, err := DB.Exec(`DROP TABLE IF EXISTS profile`); err != nil {
			log.Printf("db: drop profile: %v", err)
		}

		if _, err := DB.Exec(`ALTER TABLE profile_new RENAME TO profile`); err != nil {
			log.Printf("db: rename profile: %v", err)
		}
	}

	return nil
}
