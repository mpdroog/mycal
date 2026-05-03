package models

import (
	"log"
	"strconv"
	"strings"
)

// SearchFoods searches foods by name pattern and returns unified SearchItems.
func SearchFoods(pattern string, limit int) ([]SearchItem, error) {
	rows, err := DB.Query(`
		SELECT f.id, f.name, f.serving_type, f.serving_size,
		       COALESCE(SUM(CAST(i.calories * (fi.amount_grams / 100.0) AS INTEGER)), 0),
		       COALESCE(SUM(i.protein * (fi.amount_grams / 100.0)), 0),
		       COALESCE(SUM(i.carbs * (fi.amount_grams / 100.0)), 0),
		       COALESCE(SUM(i.fat * (fi.amount_grams / 100.0)), 0)
		FROM foods f
		LEFT JOIN food_ingredients fi ON f.id = fi.food_id
		LEFT JOIN ingredients i ON fi.ingredient_id = i.id
		WHERE f.name LIKE ? AND f.deleted_at IS NULL
		GROUP BY f.id, f.name, f.serving_type, f.serving_size
		ORDER BY f.name
		LIMIT ?
	`, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchItem
	for rows.Next() {
		var item SearchItem
		item.Type = "food"
		if err := rows.Scan(&item.ID, &item.Name, &item.ServingType, &item.ServingSize,
			&item.Calories, &item.Protein, &item.Carbs, &item.Fat); err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

// SearchIngredients searches ingredients by name pattern and returns unified SearchItems.
func SearchIngredients(pattern string, limit int) ([]SearchItem, error) {
	rows, err := DB.Query(`
		SELECT id, name, calories, protein, carbs, fat, serving_type, serving_size
		FROM ingredients
		WHERE name LIKE ? AND deleted_at IS NULL
		ORDER BY name
		LIMIT ?
	`, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchItem
	for rows.Next() {
		var item SearchItem
		item.Type = "ingredient"
		if err := rows.Scan(&item.ID, &item.Name, &item.Calories, &item.Protein,
			&item.Carbs, &item.Fat, &item.ServingType, &item.ServingSize); err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

// SearchAll searches both foods and ingredients, returning combined results.
func SearchAll(query string, limitEach int) ([]SearchItem, error) {
	if query == "" {
		return nil, nil
	}

	pattern := "%" + query + "%"
	var results []SearchItem

	foods, err := SearchFoods(pattern, limitEach)
	if err != nil {
		return nil, err
	}
	results = append(results, foods...)

	ingredients, err := SearchIngredients(pattern, limitEach)
	if err != nil {
		return nil, err
	}
	results = append(results, ingredients...)

	return results, nil
}

// GenerateTrigrams splits a string into 3-character chunks.
// "coffee" → ["cof", "off", "ffe", "fee"]
func GenerateTrigrams(s string) []string {
	s = normalizeName(s)
	if len(s) < 3 {
		if s != "" {
			return []string{s}
		}

		return nil
	}

	var trigrams []string
	runes := []rune(s)
	for i := 0; i <= len(runes)-3; i++ {
		trigrams = append(trigrams, string(runes[i:i+3]))
	}
	return trigrams
}

// IndexItem stores trigrams for a food or ingredient.
func IndexItem(itemType string, itemID int64, name string) error {
	// Remove old trigrams first
	if err := RemoveItemIndex(itemType, itemID); err != nil {
		return err
	}

	trigrams := GenerateTrigrams(name)
	for _, tri := range trigrams {
		_, err := DB.Exec(
			`INSERT OR IGNORE INTO search_trigrams (trigram, item_type, item_id) VALUES (?, ?, ?)`,
			tri, itemType, itemID,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveItemIndex removes all trigrams for an item.
func RemoveItemIndex(itemType string, itemID int64) error {
	_, err := DB.Exec(
		`DELETE FROM search_trigrams WHERE item_type = ? AND item_id = ?`,
		itemType, itemID,
	)
	return err
}

// FuzzySearch finds items by trigram similarity.
// For short queries (< 3 chars), falls back to prefix matching.
// If no trigram matches found, falls back to contains matching.
func FuzzySearch(query string, limit int) ([]SearchItem, error) {
	if query == "" {
		return nil, nil
	}

	// For very short queries, use prefix matching instead of trigrams
	if len(query) < 3 {
		return searchByPrefix(query, limit)
	}

	trigrams := GenerateTrigrams(query)
	if len(trigrams) == 0 {
		return nil, nil
	}

	results, err := fuzzySearchByTrigrams(trigrams, limit)
	if err != nil {
		return nil, err
	}

	// Fallback to contains search if no trigram matches
	if len(results) == 0 {
		return searchByContains(query, limit)
	}

	return results, nil
}

// fuzzySearchByTrigrams performs the core trigram-based search.
func fuzzySearchByTrigrams(trigrams []string, limit int) ([]SearchItem, error) {
	// Build placeholders for IN clause
	placeholders := make([]string, len(trigrams))
	args := make([]interface{}, len(trigrams)+1)
	for i, tri := range trigrams {
		placeholders[i] = "?"
		args[i] = tri
	}
	args[len(trigrams)] = limit

	// Find items with most matching trigrams
	rows, err := DB.Query(`
		SELECT item_type, item_id, COUNT(*) as matches
		FROM search_trigrams
		WHERE trigram IN (`+strings.Join(placeholders, ",")+`)
		GROUP BY item_type, item_id
		ORDER BY matches DESC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Collect item IDs by type
	var foodIDs, ingredientIDs []int64
	for rows.Next() {
		var itemType string
		var itemID int64
		var matches int
		if err := rows.Scan(&itemType, &itemID, &matches); err != nil {
			return nil, err
		}
		if itemType == "food" {
			foodIDs = append(foodIDs, itemID)
		} else {
			ingredientIDs = append(ingredientIDs, itemID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Fetch full details for matched items
	var results []SearchItem

	for _, id := range foodIDs {
		items, err := searchFoodByID(id)
		if err != nil {
			continue
		}
		results = append(results, items...)
	}

	for _, id := range ingredientIDs {
		items, err := searchIngredientByID(id)
		if err != nil {
			continue
		}
		results = append(results, items...)
	}

	return results, nil
}

func searchFoodByID(id int64) ([]SearchItem, error) {
	var item SearchItem
	item.Type = "food"
	err := DB.QueryRow(`
		SELECT f.id, f.name, f.serving_type, f.serving_size,
		       COALESCE(SUM(CAST(i.calories * (fi.amount_grams / 100.0) AS INTEGER)), 0),
		       COALESCE(SUM(i.protein * (fi.amount_grams / 100.0)), 0),
		       COALESCE(SUM(i.carbs * (fi.amount_grams / 100.0)), 0),
		       COALESCE(SUM(i.fat * (fi.amount_grams / 100.0)), 0)
		FROM foods f
		LEFT JOIN food_ingredients fi ON f.id = fi.food_id
		LEFT JOIN ingredients i ON fi.ingredient_id = i.id
		WHERE f.id = ? AND f.deleted_at IS NULL
		GROUP BY f.id, f.name, f.serving_type, f.serving_size
	`, id).Scan(&item.ID, &item.Name, &item.ServingType, &item.ServingSize,
		&item.Calories, &item.Protein, &item.Carbs, &item.Fat)
	if err != nil {
		return nil, err
	}
	return []SearchItem{item}, nil
}

func searchIngredientByID(id int64) ([]SearchItem, error) {
	var item SearchItem
	item.Type = "ingredient"
	err := DB.QueryRow(`
		SELECT id, name, calories, protein, carbs, fat, serving_type, serving_size
		FROM ingredients WHERE id = ? AND deleted_at IS NULL
	`, id).Scan(&item.ID, &item.Name, &item.Calories, &item.Protein,
		&item.Carbs, &item.Fat, &item.ServingType, &item.ServingSize)
	if err != nil {
		return nil, err
	}
	return []SearchItem{item}, nil
}

// searchByPrefix handles short queries using SQL LIKE.
func searchByPrefix(query string, limit int) ([]SearchItem, error) {
	pattern := strings.ToLower(query) + "%"

	var results []SearchItem

	// Search ingredients
	rows, err := DB.Query(`
		SELECT id, name, calories, protein, carbs, fat, serving_type, serving_size
		FROM ingredients
		WHERE LOWER(name) LIKE ? AND deleted_at IS NULL
		ORDER BY name
		LIMIT ?
	`, pattern, limit/2)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item SearchItem
		item.Type = "ingredient"

		scanErr := rows.Scan(&item.ID, &item.Name, &item.Calories, &item.Protein,
			&item.Carbs, &item.Fat, &item.ServingType, &item.ServingSize)
		if scanErr != nil {
			return nil, scanErr
		}

		results = append(results, item)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	// Search foods
	foodRows, err := DB.Query(`
		SELECT f.id, f.name, f.serving_type, f.serving_size,
		       COALESCE(SUM(CAST(i.calories * (fi.amount_grams / 100.0) AS INTEGER)), 0),
		       COALESCE(SUM(i.protein * (fi.amount_grams / 100.0)), 0),
		       COALESCE(SUM(i.carbs * (fi.amount_grams / 100.0)), 0),
		       COALESCE(SUM(i.fat * (fi.amount_grams / 100.0)), 0)
		FROM foods f
		LEFT JOIN food_ingredients fi ON f.id = fi.food_id
		LEFT JOIN ingredients i ON fi.ingredient_id = i.id
		WHERE LOWER(f.name) LIKE ? AND f.deleted_at IS NULL
		GROUP BY f.id, f.name, f.serving_type, f.serving_size
		ORDER BY f.name
		LIMIT ?
	`, pattern, limit/2)
	if err != nil {
		return nil, err
	}
	defer foodRows.Close()

	for foodRows.Next() {
		var item SearchItem
		item.Type = "food"
		if err := foodRows.Scan(&item.ID, &item.Name, &item.ServingType, &item.ServingSize,
			&item.Calories, &item.Protein, &item.Carbs, &item.Fat); err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	return results, foodRows.Err()
}

// searchByContains searches using SQL LIKE %query% when trigrams fail.
// Also tries 2-char substrings to catch typos like "xof" matching "coffee".
func searchByContains(query string, limit int) ([]SearchItem, error) {
	query = strings.ToLower(query)

	// Try full query first, then 2-char substrings
	patterns := []string{"%" + query + "%"}
	if len(query) >= 3 {
		// Add 2-char substrings (e.g., "xof" → "of" which matches "coffee")
		for i := 0; i <= len(query)-2; i++ {
			substr := query[i : i+2]
			patterns = append(patterns, "%"+substr+"%")
		}
	}

	seen := make(map[string]bool) // Deduplicate by "type:id"
	var results []SearchItem

	for _, pattern := range patterns {
		items, err := searchByPattern(pattern, limit-len(results))
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			key := item.Type + ":" + strconv.FormatInt(item.ID, 10)
			if !seen[key] {
				seen[key] = true
				results = append(results, item)
			}
		}
		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

// searchByPattern searches using a single LIKE pattern.
func searchByPattern(pattern string, limit int) ([]SearchItem, error) {
	if limit <= 0 {
		return nil, nil
	}

	var results []SearchItem

	// Search ingredients
	rows, err := DB.Query(`
		SELECT id, name, calories, protein, carbs, fat, serving_type, serving_size
		FROM ingredients
		WHERE LOWER(name) LIKE ? AND deleted_at IS NULL
		ORDER BY name
		LIMIT ?
	`, pattern, limit/2)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item SearchItem
		item.Type = "ingredient"

		scanErr := rows.Scan(&item.ID, &item.Name, &item.Calories, &item.Protein,
			&item.Carbs, &item.Fat, &item.ServingType, &item.ServingSize)
		if scanErr != nil {
			return nil, scanErr
		}

		results = append(results, item)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	// Search foods
	foodRows, err := DB.Query(`
		SELECT f.id, f.name, f.serving_type, f.serving_size,
		       COALESCE(SUM(CAST(i.calories * (fi.amount_grams / 100.0) AS INTEGER)), 0),
		       COALESCE(SUM(i.protein * (fi.amount_grams / 100.0)), 0),
		       COALESCE(SUM(i.carbs * (fi.amount_grams / 100.0)), 0),
		       COALESCE(SUM(i.fat * (fi.amount_grams / 100.0)), 0)
		FROM foods f
		LEFT JOIN food_ingredients fi ON f.id = fi.food_id
		LEFT JOIN ingredients i ON fi.ingredient_id = i.id
		WHERE LOWER(f.name) LIKE ? AND f.deleted_at IS NULL
		GROUP BY f.id, f.name, f.serving_type, f.serving_size
		ORDER BY f.name
		LIMIT ?
	`, pattern, limit/2)
	if err != nil {
		return nil, err
	}
	defer foodRows.Close()

	for foodRows.Next() {
		var item SearchItem
		item.Type = "food"
		if err := foodRows.Scan(&item.ID, &item.Name, &item.ServingType, &item.ServingSize,
			&item.Calories, &item.Protein, &item.Carbs, &item.Fat); err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	return results, foodRows.Err()
}

// IndexAllItems indexes all existing foods and ingredients.
// Call this once during migration.
func IndexAllItems() error {
	// Collect all ingredients first (close cursor before writing)
	type item struct {
		id   int64
		name string
	}

	var ingredients []item

	rows, err := DB.Query(`SELECT id, name FROM ingredients WHERE deleted_at IS NULL`)
	if err != nil {
		return err
	}

	for rows.Next() {
		var it item

		scanErr := rows.Scan(&it.id, &it.name)
		if scanErr != nil {
			//nolint:sqlclosecheck // Must close before writes to avoid SQLite BUSY
			if closeErr := rows.Close(); closeErr != nil {
				log.Printf("IndexAllItems: rows.Close error: %v", closeErr)
			}
			return scanErr
		}

		ingredients = append(ingredients, it)
	}

	if closeErr := rows.Close(); closeErr != nil {
		return closeErr
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return rowsErr
	}

	// Now index ingredients (cursor is closed)
	for _, it := range ingredients {
		indexErr := IndexItem("ingredient", it.id, it.name)
		if indexErr != nil {
			return indexErr
		}
	}

	// Collect all foods
	var foods []item
	foodRows, err := DB.Query(`SELECT id, name FROM foods WHERE deleted_at IS NULL`)
	if err != nil {
		return err
	}
	for foodRows.Next() {
		var it item

		scanErr := foodRows.Scan(&it.id, &it.name)
		if scanErr != nil {
			//nolint:sqlclosecheck // Must close before writes to avoid SQLite BUSY
			if closeErr := foodRows.Close(); closeErr != nil {
				log.Printf("IndexAllItems: foodRows.Close error: %v", closeErr)
			}
			return scanErr
		}

		foods = append(foods, it)
	}

	if closeErr := foodRows.Close(); closeErr != nil {
		return closeErr
	}

	if foodRowsErr := foodRows.Err(); foodRowsErr != nil {
		return foodRowsErr
	}

	// Now index foods (cursor is closed)
	for _, it := range foods {
		indexErr := IndexItem("food", it.id, it.name)
		if indexErr != nil {
			return indexErr
		}
	}

	return nil
}
