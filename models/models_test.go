package models

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMain(m *testing.M) {
	// Setup in-memory database
	var err error
	DB, err = sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		panic(err)
	}

	// Create tables
	_, err = DB.Exec(`
		CREATE TABLE search_trigrams (
			trigram TEXT NOT NULL,
			item_type TEXT NOT NULL,
			item_id INTEGER NOT NULL,
			PRIMARY KEY (trigram, item_type, item_id)
		)
	`)
	if err != nil {
		panic(err)
	}

	_, err = DB.Exec(`
		CREATE TABLE ingredients (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			calories INTEGER NOT NULL DEFAULT 0,
			protein REAL NOT NULL DEFAULT 0,
			carbs REAL NOT NULL DEFAULT 0,
			fat REAL NOT NULL DEFAULT 0,
			serving_size TEXT NOT NULL DEFAULT '100g',
			serving_type TEXT NOT NULL DEFAULT 'weight',
			deleted_at DATETIME DEFAULT NULL
		)
	`)
	if err != nil {
		panic(err)
	}

	_, err = DB.Exec(`
		CREATE TABLE foods (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			serving_type TEXT NOT NULL DEFAULT 'weight',
			serving_size TEXT NOT NULL DEFAULT '100g',
			deleted_at DATETIME DEFAULT NULL
		)
	`)
	if err != nil {
		panic(err)
	}

	_, err = DB.Exec(`
		CREATE TABLE food_ingredients (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			food_id INTEGER NOT NULL,
			ingredient_id INTEGER NOT NULL,
			amount_grams REAL NOT NULL DEFAULT 100
		)
	`)
	if err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func TestGenerateTrigrams(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"coffee", []string{"cof", "off", "ffe", "fee"}},
		{"ab", []string{"ab"}},  // Too short, returns as-is
		{"a", []string{"a"}},    // Single char
		{"", nil},               // Empty
		{"ABC", []string{"abc"}}, // Lowercased
		{"Café", []string{"caf", "afe"}}, // Accents removed
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := GenerateTrigrams(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("GenerateTrigrams(%q) = %v, want %v", tt.input, result, tt.expected)
				return
			}
			for i, tri := range result {
				if tri != tt.expected[i] {
					t.Errorf("GenerateTrigrams(%q)[%d] = %q, want %q", tt.input, i, tri, tt.expected[i])
				}
			}
		})
	}
}

func TestFuzzySearchMatchesTypos(t *testing.T) {
	// Clear tables
	DB.Exec("DELETE FROM search_trigrams")
	DB.Exec("DELETE FROM ingredients")

	// Insert test ingredient
	result, err := DB.Exec(`INSERT INTO ingredients (name, calories) VALUES ('Coffee', 2)`)
	if err != nil {
		t.Fatalf("Failed to insert ingredient: %v", err)
	}
	id, _ := result.LastInsertId()

	// Index it
	if err := IndexItem("ingredient", id, "Coffee"); err != nil {
		t.Fatalf("IndexItem failed: %v", err)
	}

	// Test cases: queries that should match "coffee"
	queries := []string{
		"coffee",  // Exact
		"Coffee",  // Case insensitive
		"offee",   // Missing first char
		"coffe",   // Missing last char
		"cofee",   // Missing middle char
		"xoffee",  // Wrong first char
		"offe",    // Substring via contains fallback
		"xof",     // "of" substring matches "coffee"
	}

	for _, q := range queries {
		t.Run(q, func(t *testing.T) {
			results, err := FuzzySearch(q, 10)
			if err != nil {
				t.Fatalf("FuzzySearch(%q) error: %v", q, err)
			}
			if len(results) == 0 {
				t.Errorf("FuzzySearch(%q) returned no results, expected to match 'Coffee'", q)
			} else if results[0].Name != "Coffee" {
				t.Errorf("FuzzySearch(%q) = %q, want 'Coffee'", q, results[0].Name)
			}
		})
	}
}

func TestFuzzySearchNoMatchForUnrelated(t *testing.T) {
	// Clear tables
	DB.Exec("DELETE FROM search_trigrams")
	DB.Exec("DELETE FROM ingredients")

	// Insert test ingredient
	result, _ := DB.Exec(`INSERT INTO ingredients (name, calories) VALUES ('Coffee', 2)`)
	id, _ := result.LastInsertId()
	IndexItem("ingredient", id, "Coffee")

	// These should NOT match "coffee"
	queries := []string{
		"xyz",
		"banana",
		"milk",
	}

	for _, q := range queries {
		t.Run(q, func(t *testing.T) {
			results, err := FuzzySearch(q, 10)
			if err != nil {
				t.Fatalf("FuzzySearch(%q) error: %v", q, err)
			}
			if len(results) > 0 {
				t.Errorf("FuzzySearch(%q) matched %q, expected no match", q, results[0].Name)
			}
		})
	}
}

func TestIndexItemAndRemove(t *testing.T) {
	// Clear tables
	DB.Exec("DELETE FROM search_trigrams")

	// Index an item
	err := IndexItem("ingredient", 1, "Apple")
	if err != nil {
		t.Fatalf("IndexItem failed: %v", err)
	}

	// Verify trigrams exist
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM search_trigrams WHERE item_type = 'ingredient' AND item_id = 1").Scan(&count)
	if count == 0 {
		t.Error("Expected trigrams to be indexed")
	}

	// Remove index
	err = RemoveItemIndex("ingredient", 1)
	if err != nil {
		t.Fatalf("RemoveItemIndex failed: %v", err)
	}

	// Verify trigrams removed
	DB.QueryRow("SELECT COUNT(*) FROM search_trigrams WHERE item_type = 'ingredient' AND item_id = 1").Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 trigrams after removal, got %d", count)
	}
}

func TestFuzzySearchReturnsCorrectType(t *testing.T) {
	// Clear tables
	DB.Exec("DELETE FROM search_trigrams")
	DB.Exec("DELETE FROM ingredients")
	DB.Exec("DELETE FROM foods")

	// Insert ingredient and food with similar names
	ingResult, _ := DB.Exec(`INSERT INTO ingredients (name, calories) VALUES ('Chicken Breast', 165)`)
	ingID, _ := ingResult.LastInsertId()
	IndexItem("ingredient", ingID, "Chicken Breast")

	foodResult, _ := DB.Exec(`INSERT INTO foods (name) VALUES ('Chicken Salad')`)
	foodID, _ := foodResult.LastInsertId()
	IndexItem("food", foodID, "Chicken Salad")

	// Search for "chicken"
	results, err := FuzzySearch("chicken", 10)
	if err != nil {
		t.Fatalf("FuzzySearch error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Verify types are correctly set
	hasIngredient := false
	hasFood := false
	for _, r := range results {
		if r.Type == "ingredient" && r.Name == "Chicken Breast" {
			hasIngredient = true
		}
		if r.Type == "food" && r.Name == "Chicken Salad" {
			hasFood = true
		}
	}

	if !hasIngredient {
		t.Error("Expected to find ingredient 'Chicken Breast'")
	}
	if !hasFood {
		t.Error("Expected to find food 'Chicken Salad'")
	}
}

func TestFuzzySearchEmptyQuery(t *testing.T) {
	results, err := FuzzySearch("", 10)
	if err != nil {
		t.Fatalf("FuzzySearch error: %v", err)
	}
	if results != nil {
		t.Errorf("Expected nil for empty query, got %v", results)
	}
}

func TestFuzzySearchShortQuery(t *testing.T) {
	// Clear and setup
	DB.Exec("DELETE FROM search_trigrams")
	DB.Exec("DELETE FROM ingredients")

	result, _ := DB.Exec(`INSERT INTO ingredients (name, calories) VALUES ('Egg', 78)`)
	id, _ := result.LastInsertId()
	IndexItem("ingredient", id, "Egg")

	// Search with 2-char query (becomes single trigram)
	results, err := FuzzySearch("eg", 10)
	if err != nil {
		t.Fatalf("FuzzySearch error: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected to match 'Egg' with query 'eg'")
	}
}
