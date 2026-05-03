package models

import (
	"errors"
)

// Ingredient queries

// GetIngredient returns a single ingredient by ID.
func GetIngredient(id int64) (*Ingredient, error) {
	var i Ingredient
	err := DB.QueryRow(`
		SELECT id, name, calories, protein, carbs, fat, serving_size, serving_type, created_at
		FROM ingredients WHERE id = ?
	`, id).Scan(&i.ID, &i.Name, &i.Calories, &i.Protein, &i.Carbs, &i.Fat,
		&i.ServingSize, &i.ServingType, &i.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

// ListIngredients returns paginated ingredients with optional search.
func ListIngredients(query string, limit, offset int) ([]Ingredient, int, error) {
	var totalCount int

	var countArgs []interface{}

	countSQL := `SELECT COUNT(*) FROM ingredients WHERE deleted_at IS NULL`

	if query != "" {
		countSQL += ` AND name LIKE ?`
		countArgs = append(countArgs, "%"+query+"%")
	}

	if err := DB.QueryRow(countSQL, countArgs...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	listSQL := `
		SELECT id, name, calories, protein, carbs, fat, serving_size, serving_type, created_at
		FROM ingredients WHERE deleted_at IS NULL`

	var listArgs []interface{}

	if query != "" {
		listSQL += ` AND name LIKE ?`
		listArgs = append(listArgs, "%"+query+"%")
	}

	listSQL += ` ORDER BY name ASC LIMIT ? OFFSET ?`
	listArgs = append(listArgs, limit, offset)

	rows, err := DB.Query(listSQL, listArgs...)
	if err != nil {
		return nil, 0, err
	}

	defer rows.Close()

	var ingredients []Ingredient

	for rows.Next() {
		var i Ingredient

		if err := rows.Scan(&i.ID, &i.Name, &i.Calories, &i.Protein, &i.Carbs, &i.Fat,
			&i.ServingSize, &i.ServingType, &i.CreatedAt); err != nil {
			return nil, 0, err
		}

		ingredients = append(ingredients, i)
	}

	return ingredients, totalCount, rows.Err()
}

// GetAllIngredients returns all non-deleted ingredients.
func GetAllIngredients() ([]Ingredient, error) {
	rows, err := DB.Query(`
		SELECT id, name, calories, protein, carbs, fat, serving_size, serving_type
		FROM ingredients WHERE deleted_at IS NULL ORDER BY name
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var ingredients []Ingredient

	for rows.Next() {
		var i Ingredient

		if err := rows.Scan(&i.ID, &i.Name, &i.Calories, &i.Protein, &i.Carbs, &i.Fat,
			&i.ServingSize, &i.ServingType); err != nil {
			return nil, err
		}

		ingredients = append(ingredients, i)
	}

	return ingredients, rows.Err()
}

// CreateIngredient inserts a new ingredient and returns its ID.
func CreateIngredient(i *Ingredient) (int64, error) {
	result, err := DB.Exec(`
		INSERT INTO ingredients (name, calories, protein, carbs, fat, serving_size, serving_type)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, i.Name, i.Calories, i.Protein, i.Carbs, i.Fat, i.ServingSize, i.ServingType)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// UpdateIngredient updates an existing ingredient.
func UpdateIngredient(i *Ingredient) error {
	_, err := DB.Exec(`
		UPDATE ingredients SET name = ?, calories = ?, protein = ?, carbs = ?, fat = ?, serving_size = ?, serving_type = ?
		WHERE id = ?
	`, i.Name, i.Calories, i.Protein, i.Carbs, i.Fat, i.ServingSize, i.ServingType, i.ID)
	return err
}

// DeleteIngredient soft-deletes an ingredient. Returns error if in use.
func DeleteIngredient(id int64) error {
	var count int

	err := DB.QueryRow(`
		SELECT COUNT(*) FROM food_ingredients fi
		JOIN foods f ON fi.food_id = f.id
		WHERE fi.ingredient_id = ? AND f.deleted_at IS NULL
	`, id).Scan(&count)

	if err != nil {
		return err
	}

	if count > 0 {
		return ErrInUse
	}

	_, err = DB.Exec(`UPDATE ingredients SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?`, id)

	return err
}

// RestoreIngredient restores a soft-deleted ingredient.
func RestoreIngredient(id int64) (string, error) {
	var name string

	if err := DB.QueryRow(`SELECT name FROM ingredients WHERE id = ?`, id).Scan(&name); err != nil {
		return "", err
	}

	_, err := DB.Exec(`UPDATE ingredients SET deleted_at = NULL WHERE id = ?`, id)

	return name, err
}

// IngredientExists checks if an ingredient with the given name exists.
func IngredientExists(name string) (int64, bool) {
	var id int64
	err := DB.QueryRow(`SELECT id FROM ingredients WHERE name = ?`, name).Scan(&id)
	return id, err == nil
}

// HasIngredients returns true if any ingredients exist.
func HasIngredients() bool {
	var count int

	if err := DB.QueryRow(`SELECT COUNT(*) FROM ingredients WHERE deleted_at IS NULL`).Scan(&count); err != nil {
		return false
	}

	return count > 0
}

// Food queries

// GetFood returns a food with calculated nutrition.
func GetFood(id int64) (*Food, error) {
	var f Food

	err := DB.QueryRow(`SELECT id, name, created_at FROM foods WHERE id = ?`, id).
		Scan(&f.ID, &f.Name, &f.CreatedAt)

	if err != nil {
		return nil, err
	}

	rows, err := DB.Query(`
		SELECT fi.id, fi.food_id, fi.ingredient_id, fi.amount_grams,
		       i.id, i.name, i.calories, i.protein, i.carbs, i.fat, i.serving_size
		FROM food_ingredients fi
		JOIN ingredients i ON fi.ingredient_id = i.id
		WHERE fi.food_id = ?
	`, id)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var fi FoodIngredient

		var ing Ingredient

		if err := rows.Scan(&fi.ID, &fi.FoodID, &fi.IngredientID, &fi.AmountGrams,
			&ing.ID, &ing.Name, &ing.Calories, &ing.Protein, &ing.Carbs, &ing.Fat, &ing.ServingSize); err != nil {
			return nil, err
		}

		fi.Ingredient = &ing

		f.Ingredients = append(f.Ingredients, fi)

		ratio := fi.AmountGrams / 100.0
		f.Calories += int(float64(ing.Calories) * ratio)
		f.Protein += ing.Protein * ratio
		f.Carbs += ing.Carbs * ratio
		f.Fat += ing.Fat * ratio
	}

	return &f, rows.Err()
}

// ListFoods returns paginated foods with calculated nutrition.
func ListFoods(query string, limit, offset int) ([]Food, int, error) {
	var totalCount int

	var countArgs []interface{}

	countSQL := `SELECT COUNT(*) FROM foods WHERE deleted_at IS NULL`

	if query != "" {
		countSQL += ` AND name LIKE ?`
		countArgs = append(countArgs, "%"+query+"%")
	}

	if err := DB.QueryRow(countSQL, countArgs...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	listSQL := `
		SELECT f.id, f.name, f.serving_type, f.serving_size, f.created_at,
		       COALESCE(SUM(CAST(i.calories * (fi.amount_grams / 100.0) AS INTEGER)), 0) as calories,
		       COALESCE(SUM(i.protein * (fi.amount_grams / 100.0)), 0) as protein,
		       COALESCE(SUM(i.carbs * (fi.amount_grams / 100.0)), 0) as carbs,
		       COALESCE(SUM(i.fat * (fi.amount_grams / 100.0)), 0) as fat
		FROM foods f
		LEFT JOIN food_ingredients fi ON f.id = fi.food_id
		LEFT JOIN ingredients i ON fi.ingredient_id = i.id
		WHERE f.deleted_at IS NULL`

	var listArgs []interface{}

	if query != "" {
		listSQL += ` AND f.name LIKE ?`
		listArgs = append(listArgs, "%"+query+"%")
	}

	listSQL += ` GROUP BY f.id, f.name, f.serving_type, f.serving_size, f.created_at ORDER BY f.name LIMIT ? OFFSET ?`
	listArgs = append(listArgs, limit, offset)

	rows, err := DB.Query(listSQL, listArgs...)
	if err != nil {
		return nil, 0, err
	}

	defer rows.Close()

	var foods []Food

	for rows.Next() {
		var f Food

		if err := rows.Scan(&f.ID, &f.Name, &f.ServingType, &f.ServingSize, &f.CreatedAt,
			&f.Calories, &f.Protein, &f.Carbs, &f.Fat); err != nil {
			return nil, 0, err
		}

		foods = append(foods, f)
	}

	return foods, totalCount, rows.Err()
}

// GetAllFoods returns all foods with calculated nutrition.
func GetAllFoods() ([]Food, error) {
	rows, err := DB.Query(`
		SELECT f.id, f.name, f.serving_type, f.serving_size, f.created_at,
		       COALESCE(SUM(CAST(i.calories * (fi.amount_grams / 100.0) AS INTEGER)), 0) as calories,
		       COALESCE(SUM(i.protein * (fi.amount_grams / 100.0)), 0) as protein,
		       COALESCE(SUM(i.carbs * (fi.amount_grams / 100.0)), 0) as carbs,
		       COALESCE(SUM(i.fat * (fi.amount_grams / 100.0)), 0) as fat
		FROM foods f
		LEFT JOIN food_ingredients fi ON f.id = fi.food_id
		LEFT JOIN ingredients i ON fi.ingredient_id = i.id
		WHERE f.deleted_at IS NULL
		GROUP BY f.id, f.name, f.serving_type, f.serving_size, f.created_at
		ORDER BY f.name
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var foods []Food

	for rows.Next() {
		var f Food

		if err := rows.Scan(&f.ID, &f.Name, &f.ServingType, &f.ServingSize, &f.CreatedAt,
			&f.Calories, &f.Protein, &f.Carbs, &f.Fat); err != nil {
			return nil, err
		}

		foods = append(foods, f)
	}

	return foods, rows.Err()
}

// CreateFood creates a new food with ingredients.
func CreateFood(name string, ingredients []FoodIngredient) (int64, error) {
	result, err := DB.Exec(`INSERT INTO foods (name) VALUES (?)`, name)
	if err != nil {
		return 0, err
	}
	foodID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	for _, fi := range ingredients {
		_, err := DB.Exec(`
			INSERT INTO food_ingredients (food_id, ingredient_id, amount_grams)
			VALUES (?, ?, ?)
		`, foodID, fi.IngredientID, fi.AmountGrams)
		if err != nil {
			return 0, err
		}
	}
	return foodID, nil
}

// UpdateFood updates a food and its ingredients.
func UpdateFood(id int64, name string, ingredients []FoodIngredient) error {
	if _, err := DB.Exec(`UPDATE foods SET name = ? WHERE id = ?`, name, id); err != nil {
		return err
	}

	if _, err := DB.Exec(`DELETE FROM food_ingredients WHERE food_id = ?`, id); err != nil {
		return err
	}

	for _, fi := range ingredients {
		_, err := DB.Exec(`
			INSERT INTO food_ingredients (food_id, ingredient_id, amount_grams)
			VALUES (?, ?, ?)
		`, id, fi.IngredientID, fi.AmountGrams)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteFood soft-deletes a food. Returns error if in use.
func DeleteFood(id int64) error {
	var count int

	err := DB.QueryRow(`SELECT COUNT(*) FROM entries WHERE food_id = ? AND deleted_at IS NULL`, id).Scan(&count)

	if err != nil {
		return err
	}

	if count > 0 {
		return ErrInUse
	}

	_, err = DB.Exec(`UPDATE foods SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?`, id)

	return err
}

// RestoreFood restores a soft-deleted food.
func RestoreFood(id int64) (string, error) {
	var name string

	if err := DB.QueryRow(`SELECT name FROM foods WHERE id = ?`, id).Scan(&name); err != nil {
		return "", err
	}

	_, err := DB.Exec(`UPDATE foods SET deleted_at = NULL WHERE id = ?`, id)

	return name, err
}

// FindOrCreateSimpleFood finds or creates a simple food from an ingredient.
func FindOrCreateSimpleFood(ingredientID int64) (int64, string, error) {
	var name, servingType, servingSize string

	err := DB.QueryRow(`SELECT name, serving_type, serving_size FROM ingredients WHERE id = ?`,
		ingredientID).Scan(&name, &servingType, &servingSize)

	if err != nil {
		return 0, "", err
	}

	// Check if simple food already exists
	var foodID int64

	err = DB.QueryRow(`
		SELECT f.id FROM foods f
		JOIN food_ingredients fi ON f.id = fi.food_id
		WHERE f.name = ? AND fi.ingredient_id = ? AND fi.amount_grams = 100
		GROUP BY f.id HAVING COUNT(*) = 1
	`, name, ingredientID).Scan(&foodID)

	if err == nil {
		return foodID, servingType, nil
	}

	// Create simple food
	result, err := DB.Exec(`INSERT INTO foods (name, serving_type, serving_size) VALUES (?, ?, ?)`,
		name, servingType, servingSize)
	if err != nil {
		return 0, "", err
	}

	foodID, err = result.LastInsertId()

	if err != nil {
		return 0, "", err
	}

	_, err = DB.Exec(`INSERT INTO food_ingredients (food_id, ingredient_id, amount_grams) VALUES (?, ?, 100)`,
		foodID, ingredientID)

	return foodID, servingType, err
}

// Entry queries

// GetEntry returns a single entry with food details.
func GetEntry(id, userID int64) (*Entry, error) {
	var e Entry
	var f Food
	err := DB.QueryRow(`
		SELECT e.id, e.food_id, e.date, e.meal, e.servings, COALESCE(e.notes, ''),
		       f.id, f.name
		FROM entries e
		JOIN foods f ON e.food_id = f.id
		WHERE e.id = ? AND e.user_id = ?
	`, id, userID).Scan(&e.ID, &e.FoodID, &e.Date, &e.Meal, &e.Servings, &e.Notes,
		&f.ID, &f.Name)
	if err != nil {
		return nil, err
	}
	e.Food = &f
	return &e, nil
}

// GetDaySummary returns all entries for a date with nutrition totals.
func GetDaySummary(date string, userID int64) DaySummary {
	summary := DaySummary{Date: date}

	rows, err := DB.Query(`
		SELECT e.id, e.food_id, e.date, e.meal, e.servings, COALESCE(e.notes, ''),
		       f.id, f.name, f.serving_type, f.serving_size,
		       COALESCE(SUM(CAST(i.calories * (fi.amount_grams / 100.0) AS INTEGER)), 0) as calories,
		       COALESCE(SUM(i.protein * (fi.amount_grams / 100.0)), 0) as protein,
		       COALESCE(SUM(i.carbs * (fi.amount_grams / 100.0)), 0) as carbs,
		       COALESCE(SUM(i.fat * (fi.amount_grams / 100.0)), 0) as fat
		FROM entries e
		JOIN foods f ON e.food_id = f.id
		LEFT JOIN food_ingredients fi ON f.id = fi.food_id
		LEFT JOIN ingredients i ON fi.ingredient_id = i.id
		WHERE e.date = ? AND e.deleted_at IS NULL AND e.user_id = ?
		GROUP BY e.id, e.food_id, e.date, e.meal, e.servings, e.notes, f.id, f.name, f.serving_type, f.serving_size
		ORDER BY e.created_at ASC
	`, date, userID)
	if err != nil {
		return summary
	}

	defer rows.Close()

	for rows.Next() {
		var e Entry

		var f Food

		if err := rows.Scan(&e.ID, &e.FoodID, &e.Date, &e.Meal, &e.Servings, &e.Notes,
			&f.ID, &f.Name, &f.ServingType, &f.ServingSize, &f.Calories, &f.Protein, &f.Carbs, &f.Fat); err != nil {
			continue
		}

		e.Food = &f

		summary.Calories += int(float64(f.Calories) * e.Servings)
		summary.Protein += f.Protein * e.Servings
		summary.Carbs += f.Carbs * e.Servings
		summary.Fat += f.Fat * e.Servings

		summary.Entries = append(summary.Entries, e)
	}

	// Check for iteration errors (unlikely but best practice)
	_ = rows.Err() //nolint:errcheck // Summary function has no error return

	return summary
}

// CreateEntry creates a new diary entry.
func CreateEntry(e *Entry) error {
	_, err := DB.Exec(`
		INSERT INTO entries (food_id, date, meal, servings, notes, user_id)
		VALUES (?, ?, ?, ?, ?, ?)
	`, e.FoodID, e.Date, e.Meal, e.Servings, e.Notes, e.UserID)
	return err
}

// UpdateEntry updates an existing entry.
func UpdateEntry(e *Entry) error {
	result, err := DB.Exec(`
		UPDATE entries SET food_id = ?, meal = ?, servings = ?, notes = ?
		WHERE id = ? AND user_id = ?
	`, e.FoodID, e.Meal, e.Servings, e.Notes, e.ID, e.UserID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// UpdateEntryServings updates only the servings of an entry.
func UpdateEntryServings(id, userID int64, servings float64) error {
	result, err := DB.Exec(`UPDATE entries SET servings = ? WHERE id = ? AND user_id = ?`, servings, id, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteEntry soft-deletes an entry. Returns the entry date.
func DeleteEntry(id, userID int64) (string, error) {
	var date string
	if err := DB.QueryRow(`SELECT date FROM entries WHERE id = ? AND user_id = ?`, id, userID).Scan(&date); err != nil {
		return "", err
	}
	_, err := DB.Exec(`UPDATE entries SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?`, id, userID)
	return date, err
}

// RestoreEntry restores a soft-deleted entry. Returns the entry date.
func RestoreEntry(id, userID int64) (string, error) {
	var date string

	if err := DB.QueryRow(`SELECT date FROM entries WHERE id = ? AND user_id = ?`, id, userID).Scan(&date); err != nil {
		return "", err
	}

	_, err := DB.Exec(`UPDATE entries SET deleted_at = NULL WHERE id = ? AND user_id = ?`, id, userID)

	return date, err
}

// GetEntryDate returns the date of an entry.
func GetEntryDate(id, userID int64) (string, error) {
	var date string
	err := DB.QueryRow(`SELECT date FROM entries WHERE id = ? AND user_id = ?`, id, userID).Scan(&date)
	return date, err
}

// Profile queries

// GetProfile returns the profile for a user, or defaults if not found.
func GetProfile(userID int64) Profile {
	profile := Profile{
		UserID:       userID,
		CaloriesGoal: 2000,
		ProteinGoal:  150,
		CarbsGoal:    250,
		FatGoal:      65,
	}

	// Error is ignored intentionally - we return defaults if profile not found
	//nolint:errcheck // Returns defaults if profile not found
	DB.QueryRow(`
		SELECT calories_goal, protein_goal, carbs_goal, fat_goal
		FROM profile WHERE user_id = ?
	`, userID).Scan(&profile.CaloriesGoal, &profile.ProteinGoal, &profile.CarbsGoal, &profile.FatGoal)

	return profile
}

// SaveProfile creates or updates a user profile.
func SaveProfile(p *Profile) error {
	_, err := DB.Exec(`
		INSERT INTO profile (user_id, calories_goal, protein_goal, carbs_goal, fat_goal)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			calories_goal = excluded.calories_goal,
			protein_goal = excluded.protein_goal,
			carbs_goal = excluded.carbs_goal,
			fat_goal = excluded.fat_goal
	`, p.UserID, p.CaloriesGoal, p.ProteinGoal, p.CarbsGoal, p.FatGoal)
	return err
}

// CreateDefaultProfile creates a default profile for a new user.
func CreateDefaultProfile(userID int64) error {
	_, err := DB.Exec(`
		INSERT OR IGNORE INTO profile (user_id, calories_goal, protein_goal, carbs_goal, fat_goal)
		VALUES (?, 2000, 150, 250, 65)
	`, userID)
	return err
}

// ErrInUse is returned when trying to delete an item that is in use.
var ErrInUse = errors.New("item is in use")
