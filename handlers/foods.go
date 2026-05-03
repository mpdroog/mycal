package handlers

import (
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/mpdroog/mycal/auth"
	"github.com/mpdroog/mycal/db"
	"github.com/mpdroog/mycal/models"
)

// getFoodWithNutrition loads a food and calculates its nutritional values from ingredients.
func getFoodWithNutrition(foodID int64) (*models.Food, error) {
	var f models.Food

	err := db.DB.QueryRow(`SELECT id, name, created_at FROM foods WHERE id = ?`, foodID).
		Scan(&f.ID, &f.Name, &f.CreatedAt)
	if err != nil {
		return nil, err
	}

	// Get ingredients with their amounts
	rows, err := db.DB.Query(`
		SELECT fi.id, fi.food_id, fi.ingredient_id, fi.amount_grams,
		       i.id, i.name, i.calories, i.protein, i.carbs, i.fat, i.serving_size
		FROM food_ingredients fi
		JOIN ingredients i ON fi.ingredient_id = i.id
		WHERE fi.food_id = ?
	`, foodID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var fi models.FoodIngredient
		var ing models.Ingredient

		err := rows.Scan(&fi.ID, &fi.FoodID, &fi.IngredientID, &fi.AmountGrams,
			&ing.ID, &ing.Name, &ing.Calories, &ing.Protein, &ing.Carbs, &ing.Fat, &ing.ServingSize)
		if err != nil {
			return nil, err
		}

		fi.Ingredient = &ing
		f.Ingredients = append(f.Ingredients, fi)

		// Calculate nutrition based on amount (assuming ingredient values are per 100g)
		ratio := fi.AmountGrams / 100.0
		f.Calories += int(float64(ing.Calories) * ratio)
		f.Protein += ing.Protein * ratio
		f.Carbs += ing.Carbs * ratio
		f.Fat += ing.Fat * ratio
	}

	return &f, rows.Err()
}

// getAllFoods returns all foods with their calculated nutritional values.
// Uses a single optimized query with JOINs to avoid N+1 queries.
func getAllFoods() ([]models.Food, error) {
	rows, err := db.DB.Query(`
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

	var foods []models.Food

	for rows.Next() {
		var f models.Food

		if err := rows.Scan(&f.ID, &f.Name, &f.ServingType, &f.ServingSize, &f.CreatedAt, &f.Calories, &f.Protein, &f.Carbs, &f.Fat); err != nil {
			return nil, err
		}

		foods = append(foods, f)
	}

	return foods, rows.Err()
}

func ListFoods(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := auth.GetUserFromContext(r.Context())

		// Get search and pagination params
		query := r.URL.Query().Get("q")
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		offset := (page - 1) * itemsPerPage

		// Count total foods
		var totalCount int
		var countArgs []interface{}
		countQuery := `SELECT COUNT(*) FROM foods WHERE deleted_at IS NULL`
		if query != "" {
			countQuery += ` AND name LIKE ?`
			countArgs = append(countArgs, "%"+query+"%")
		}
		if err := db.DB.QueryRow(countQuery, countArgs...).Scan(&totalCount); err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}

		// Build list query with search and pagination
		listQuery := `
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
			listQuery += ` AND f.name LIKE ?`
			listArgs = append(listArgs, "%"+query+"%")
		}
		listQuery += ` GROUP BY f.id, f.name, f.serving_type, f.serving_size, f.created_at ORDER BY f.name LIMIT ? OFFSET ?`
		listArgs = append(listArgs, itemsPerPage, offset)

		rows, err := db.DB.Query(listQuery, listArgs...)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var foods []models.Food
		for rows.Next() {
			var f models.Food
			if err := rows.Scan(&f.ID, &f.Name, &f.ServingType, &f.ServingSize, &f.CreatedAt, &f.Calories, &f.Protein, &f.Carbs, &f.Fat); err != nil {
				httpError(w, err, http.StatusInternalServerError)
				return
			}
			foods = append(foods, f)
		}
		if err := rows.Err(); err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}

		totalPages := (totalCount + itemsPerPage - 1) / itemsPerPage
		if totalPages < 1 {
			totalPages = 1
		}

		data := map[string]interface{}{
			"Title":      "Foods",
			"Foods":      foods,
			"User":       user,
			"Query":      query,
			"Page":       page,
			"TotalPages": totalPages,
			"TotalCount": totalCount,
			"HasPrev":    page > 1,
			"HasNext":    page < totalPages,
			"PrevPage":   page - 1,
			"NextPage":   page + 1,
		}

		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			httpError(w, err, http.StatusInternalServerError)
		}
	}
}

func CreateFood(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := auth.GetUserFromContext(r.Context())

		if r.Method == http.MethodGet {
			data := map[string]interface{}{
				"Title": "Add Food",
				"Food":  models.Food{},
				"User":  user,
			}

			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				httpError(w, err, http.StatusInternalServerError)
			}

			return
		}

		if err := r.ParseForm(); err != nil {
			httpError(w, err, http.StatusBadRequest)

			return
		}

		name := r.FormValue("name")
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)

			return
		}

		// Parse ingredients JSON
		ingredientsJSON := r.FormValue("ingredients")

		var foodIngredients []struct {
			IngredientID int64   `json:"ingredient_id"`
			AmountGrams  float64 `json:"amount_grams"`
		}

		if ingredientsJSON != "" {
			if err := json.Unmarshal([]byte(ingredientsJSON), &foodIngredients); err != nil {
				http.Error(w, "invalid ingredients format", http.StatusBadRequest)

				return
			}
		}

		// Insert food
		result, err := db.DB.Exec(`INSERT INTO foods (name) VALUES (?)`, name)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)

			return
		}

		foodID, err := result.LastInsertId()
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)

			return
		}

		// Insert food ingredients
		for _, fi := range foodIngredients {
			_, err := db.DB.Exec(`
				INSERT INTO food_ingredients (food_id, ingredient_id, amount_grams)
				VALUES (?, ?, ?)
			`, foodID, fi.IngredientID, fi.AmountGrams)
			if err != nil {
				httpError(w, err, http.StatusInternalServerError)

				return
			}
		}

		// Index for fuzzy search
		_ = models.IndexItem("food", foodID, name)

		http.Redirect(w, r, "/foods", http.StatusSeeOther)
	}
}

func EditFood(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := auth.GetUserFromContext(r.Context())

		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)

			return
		}

		if r.Method == http.MethodGet {
			food, err := getFoodWithNutrition(id)
			if errors.Is(err, models.ErrNotFound) {
				http.NotFound(w, r)

				return
			}

			if err != nil {
				httpError(w, err, http.StatusInternalServerError)

				return
			}

			data := map[string]interface{}{
				"Title": "Edit Food",
				"Food":  food,
				"User":  user,
			}

			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				httpError(w, err, http.StatusInternalServerError)
			}

			return
		}

		if err := r.ParseForm(); err != nil {
			httpError(w, err, http.StatusBadRequest)

			return
		}

		name := r.FormValue("name")
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)

			return
		}

		// Parse ingredients JSON
		ingredientsJSON := r.FormValue("ingredients")

		var foodIngredients []struct {
			IngredientID int64   `json:"ingredient_id"`
			AmountGrams  float64 `json:"amount_grams"`
		}

		if ingredientsJSON != "" {
			if err := json.Unmarshal([]byte(ingredientsJSON), &foodIngredients); err != nil {
				http.Error(w, "invalid ingredients format", http.StatusBadRequest)

				return
			}
		}

		// Update food name
		_, err = db.DB.Exec(`UPDATE foods SET name = ? WHERE id = ?`, name, id)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)

			return
		}

		// Delete old ingredients and insert new ones
		_, err = db.DB.Exec(`DELETE FROM food_ingredients WHERE food_id = ?`, id)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)

			return
		}

		for _, fi := range foodIngredients {
			_, err := db.DB.Exec(`
				INSERT INTO food_ingredients (food_id, ingredient_id, amount_grams)
				VALUES (?, ?, ?)
			`, id, fi.IngredientID, fi.AmountGrams)
			if err != nil {
				httpError(w, err, http.StatusInternalServerError)

				return
			}
		}

		// Re-index for fuzzy search
		_ = models.IndexItem("food", id, name)

		http.Redirect(w, r, "/foods", http.StatusSeeOther)
	}
}

func DeleteFood(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)

		return
	}

	// Check if any non-deleted entries reference this food
	var entryCount int

	err = db.DB.QueryRow("SELECT COUNT(*) FROM entries WHERE food_id = ? AND deleted_at IS NULL", id).Scan(&entryCount)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)

		return
	}

	if entryCount > 0 {
		http.Error(w, "Cannot delete food: it is used in diary entries. Delete the entries first.", http.StatusConflict)

		return
	}

	// Soft delete: set deleted_at timestamp
	_, err = db.DB.Exec("UPDATE foods SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)

		return
	}

	// Remove from search index
	_ = models.RemoveItemIndex("food", id)

	http.Redirect(w, r, "/foods?deleted=food&id="+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func RestoreFood(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)

		return
	}

	// Get name for re-indexing
	var name string
	_ = db.DB.QueryRow("SELECT name FROM foods WHERE id = ?", id).Scan(&name)

	// Clear deleted_at to restore
	_, err = db.DB.Exec("UPDATE foods SET deleted_at = NULL WHERE id = ?", id)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)

		return
	}

	// Re-index for fuzzy search
	if name != "" {
		_ = models.IndexItem("food", id, name)
	}

	http.Redirect(w, r, "/foods", http.StatusSeeOther)
}

func SearchFoods(w http.ResponseWriter, r *http.Request) {
	q := "%" + r.URL.Query().Get("q") + "%"

	rows, err := db.DB.Query(`
		SELECT f.id, f.name,
		       COALESCE(SUM(CAST(i.calories * (fi.amount_grams / 100.0) AS INTEGER)), 0) as calories,
		       COALESCE(SUM(i.protein * (fi.amount_grams / 100.0)), 0) as protein,
		       COALESCE(SUM(i.carbs * (fi.amount_grams / 100.0)), 0) as carbs,
		       COALESCE(SUM(i.fat * (fi.amount_grams / 100.0)), 0) as fat
		FROM foods f
		LEFT JOIN food_ingredients fi ON f.id = fi.food_id
		LEFT JOIN ingredients i ON fi.ingredient_id = i.id
		WHERE f.name LIKE ? AND f.deleted_at IS NULL
		GROUP BY f.id, f.name
		ORDER BY f.name
		LIMIT 10
	`, q)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)

		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "application/json")

	var foods []models.Food

	for rows.Next() {
		var f models.Food

		if err := rows.Scan(&f.ID, &f.Name, &f.Calories, &f.Protein, &f.Carbs, &f.Fat); err != nil {
			log.Printf("SearchFoods scan error: %v", err)

			continue
		}

		foods = append(foods, f)
	}

	if err := json.NewEncoder(w).Encode(foods); err != nil {
		log.Printf("SearchFoods encode error: %v", err)
	}
}
