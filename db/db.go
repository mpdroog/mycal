package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func Init(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "mycal.db")

	var err error

	DB, err = sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)&_pragma=strict(1)")
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	if err := migrate(); err != nil {
		return fmt.Errorf("migrate: %w", err)
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
		// Profile/settings table (single row for user preferences)
		`CREATE TABLE IF NOT EXISTS profile (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			calories_goal INTEGER NOT NULL DEFAULT 2000,
			protein_goal REAL NOT NULL DEFAULT 150,
			carbs_goal REAL NOT NULL DEFAULT 250,
			fat_goal REAL NOT NULL DEFAULT 65
		)`,
		// Insert default profile if not exists
		`INSERT OR IGNORE INTO profile (id, calories_goal, protein_goal, carbs_goal, fat_goal) VALUES (1, 2000, 150, 250, 65)`,
	}

	for _, m := range migrations {
		if _, err := DB.Exec(m); err != nil {
			return fmt.Errorf("execute migration: %w", err)
		}
	}

	return nil
}
