package handlers

import (
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mpdroog/mycal/auth"
	"github.com/mpdroog/mycal/db"
	"github.com/mpdroog/mycal/models"
)

func Dashboard(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := auth.GetUserFromContext(r.Context())

		date := r.URL.Query().Get("date")
		if date == "" {
			date = time.Now().Format("2006-01-02")
		} else {
			// Normalize date format (handle timestamps, etc.)
			if t, err := time.Parse(time.RFC3339, date); err == nil {
				date = t.Format("2006-01-02")
			} else if t, err := time.Parse("2006-01-02T15:04:05Z", date); err == nil {
				date = t.Format("2006-01-02")
			} else if len(date) > 10 {
				date = date[:10]
			}
		}

		summary := getDaySummary(date, user.ID)

		// Get all foods for the dropdown
		foods, err := getAllFoods()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		// Get all ingredients for the dropdown
		ingredients, err := GetAllIngredients()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		// Get profile for goals
		profile, err := GetProfileForUser(user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		data := map[string]interface{}{
			"Title":       "Today",
			"Date":        date,
			"Summary":     summary,
			"Foods":       foods,
			"Ingredients": ingredients,
			"Meals":       []string{"breakfast", "lunch", "dinner", "snack"},
			"Profile":     profile,
			"User":        user,
		}

		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func CreateEntry(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	var foodID int64

	// Track serving type for conversion
	servingType := "weight"

	// Check if ingredient_id is provided (quick add from ingredient)
	if ingredientIDStr := r.FormValue("ingredient_id"); ingredientIDStr != "" {
		ingredientID, err := strconv.ParseInt(ingredientIDStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid ingredient_id", http.StatusBadRequest)

			return
		}

		// Get ingredient name, serving_type, and serving_size
		var ingredientName, ingredientServingType, ingredientServingSize string

		err = db.DB.QueryRow("SELECT name, serving_type, serving_size FROM ingredients WHERE id = ?", ingredientID).Scan(&ingredientName, &ingredientServingType, &ingredientServingSize)
		if err != nil {
			http.Error(w, "ingredient not found", http.StatusBadRequest)

			return
		}

		servingType = ingredientServingType

		// Check if a simple food already exists for this ingredient
		err = db.DB.QueryRow(`
			SELECT f.id FROM foods f
			JOIN food_ingredients fi ON f.id = fi.food_id
			WHERE f.name = ? AND fi.ingredient_id = ? AND fi.amount_grams = 100
			GROUP BY f.id HAVING COUNT(*) = 1
		`, ingredientName, ingredientID).Scan(&foodID)

		if err != nil {
			// Create a simple food from this ingredient (100g)
			result, err := db.DB.Exec("INSERT INTO foods (name, serving_type, serving_size) VALUES (?, ?, ?)", ingredientName, ingredientServingType, ingredientServingSize)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}

			foodID, err = result.LastInsertId()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}

			// Link ingredient to food (100g)
			_, err = db.DB.Exec(`
				INSERT INTO food_ingredients (food_id, ingredient_id, amount_grams)
				VALUES (?, ?, 100)
			`, foodID, ingredientID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}
		}
	} else {
		// Regular food_id
		var err error

		foodID, err = strconv.ParseInt(r.FormValue("food_id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid food_id", http.StatusBadRequest)

			return
		}
	}

	servingsInput, err := strconv.ParseFloat(r.FormValue("servings"), 64)
	if err != nil {
		http.Error(w, "invalid servings value", http.StatusBadRequest)

		return
	}

	if servingsInput <= 0 {
		servingsInput = 100 // default to 100g for weight, 1 for unit
		if servingType == "unit" {
			servingsInput = 1
		}
	}

	// Convert input to servings based on serving type
	var servings float64
	if servingType == "weight" {
		// Input is in grams, convert to servings (grams / 100)
		servings = servingsInput / 100
	} else {
		// Input is count (unit-based), use directly
		servings = servingsInput
	}

	date := r.FormValue("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	_, err = db.DB.Exec(`
		INSERT INTO entries (food_id, date, meal, servings, notes, user_id)
		VALUES (?, ?, ?, ?, ?, ?)
	`, foodID, date, r.FormValue("meal"), servings, r.FormValue("notes"), user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/?date="+date, http.StatusSeeOther)
}

func DeleteEntry(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)

		return
	}

	// Get the date before soft-deleting (verify ownership)
	var dateRaw string

	if scanErr := db.DB.QueryRow("SELECT date FROM entries WHERE id = ? AND user_id = ?", id, user.ID).Scan(&dateRaw); scanErr != nil {
		http.Error(w, "entry not found", http.StatusNotFound)

		return
	}

	// Parse and format date (SQLite may return various formats)
	date := time.Now().Format("2006-01-02")
	if dateRaw != "" {
		if t, parseErr := time.Parse(time.RFC3339, dateRaw); parseErr == nil {
			date = t.Format("2006-01-02")
		} else if t, parseErr := time.Parse("2006-01-02", dateRaw); parseErr == nil {
			date = t.Format("2006-01-02")
		} else if len(dateRaw) >= 10 {
			date = dateRaw[:10]
		}
	}

	// Soft delete: set deleted_at timestamp
	_, err = db.DB.Exec("UPDATE entries SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?", id, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/?date="+date+"&deleted=entry&id="+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func RestoreEntry(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)

		return
	}

	// Get the date for redirect (verify ownership)
	var dateRaw string

	if scanErr := db.DB.QueryRow("SELECT date FROM entries WHERE id = ? AND user_id = ?", id, user.ID).Scan(&dateRaw); scanErr != nil {
		http.Error(w, "entry not found", http.StatusNotFound)

		return
	}

	// Clear deleted_at to restore
	_, err = db.DB.Exec("UPDATE entries SET deleted_at = NULL WHERE id = ? AND user_id = ?", id, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	// Parse and format date
	date := time.Now().Format("2006-01-02")
	if dateRaw != "" {
		if t, parseErr := time.Parse(time.RFC3339, dateRaw); parseErr == nil {
			date = t.Format("2006-01-02")
		} else if t, parseErr := time.Parse("2006-01-02", dateRaw); parseErr == nil {
			date = t.Format("2006-01-02")
		} else if len(dateRaw) >= 10 {
			date = dateRaw[:10]
		}
	}

	http.Redirect(w, r, "/?date="+date, http.StatusSeeOther)
}

func getDaySummary(date string, userID int64) models.DaySummary {
	summary := models.DaySummary{Date: date}

	// Single optimized query with JOINs to avoid N+1 queries
	rows, err := db.DB.Query(`
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
		log.Printf("getDaySummary: query failed: %v", err)

		return summary
	}
	defer rows.Close()

	for rows.Next() {
		var e models.Entry
		var f models.Food

		if err := rows.Scan(&e.ID, &e.FoodID, &e.Date, &e.Meal, &e.Servings, &e.Notes,
			&f.ID, &f.Name, &f.ServingType, &f.ServingSize, &f.Calories, &f.Protein, &f.Carbs, &f.Fat); err != nil {
			log.Printf("getDaySummary: scan failed: %v", err)

			continue
		}

		e.Food = &f

		// Calculate totals based on servings
		summary.Calories += int(float64(f.Calories) * e.Servings)
		summary.Protein += f.Protein * e.Servings
		summary.Carbs += f.Carbs * e.Servings
		summary.Fat += f.Fat * e.Servings

		summary.Entries = append(summary.Entries, e)
	}

	if err := rows.Err(); err != nil {
		log.Printf("getDaySummary: rows error: %v", err)
	}

	return summary
}

func GetEntry(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := auth.GetUserFromContext(r.Context())

		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)

			return
		}

		var e models.Entry
		var f models.Food

		err = db.DB.QueryRow(`
			SELECT e.id, e.food_id, e.date, e.meal, e.servings, COALESCE(e.notes, ''),
			       f.id, f.name
			FROM entries e
			JOIN foods f ON e.food_id = f.id
			WHERE e.id = ? AND e.user_id = ?
		`, id, user.ID).Scan(&e.ID, &e.FoodID, &e.Date, &e.Meal, &e.Servings, &e.Notes,
			&f.ID, &f.Name)
		if errors.Is(err, models.ErrNotFound) {
			http.NotFound(w, r)

			return
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		e.Food = &f

		foods, err := getAllFoods()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		data := map[string]interface{}{
			"Title": "Edit Entry",
			"Entry": e,
			"Foods": foods,
			"Meals": []string{"breakfast", "lunch", "dinner", "snack"},
			"User":  user,
		}

		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func UpdateEntryServings(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)

		return
	}

	// Support both urlencoded and multipart form data (JavaScript FormData uses multipart)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		// Fall back to regular form parsing
		if parseErr := r.ParseForm(); parseErr != nil {
			http.Error(w, parseErr.Error(), http.StatusBadRequest)

			return
		}
	}

	servings, err := strconv.ParseFloat(r.FormValue("servings"), 64)
	if err != nil {
		http.Error(w, "invalid servings value", http.StatusBadRequest)

		return
	}

	if servings <= 0 {
		servings = 0.25
	}

	result, err := db.DB.Exec(`UPDATE entries SET servings = ? WHERE id = ? AND user_id = ?`, servings, id, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "entry not found", http.StatusNotFound)

		return
	}

	w.WriteHeader(http.StatusOK)
}

func UpdateEntry(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)

		return
	}

	if parseErr := r.ParseForm(); parseErr != nil {
		http.Error(w, parseErr.Error(), http.StatusBadRequest)

		return
	}

	foodID, err := strconv.ParseInt(r.FormValue("food_id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid food_id", http.StatusBadRequest)

		return
	}

	servings, err := strconv.ParseFloat(r.FormValue("servings"), 64)
	if err != nil {
		http.Error(w, "invalid servings value", http.StatusBadRequest)

		return
	}

	if servings <= 0 {
		servings = 1
	}

	result, err := db.DB.Exec(`
		UPDATE entries SET food_id = ?, meal = ?, servings = ?, notes = ?
		WHERE id = ? AND user_id = ?
	`, foodID, r.FormValue("meal"), servings, r.FormValue("notes"), id, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "entry not found", http.StatusNotFound)

		return
	}

	// Get date for redirect
	var date string

	if scanErr := db.DB.QueryRow("SELECT date FROM entries WHERE id = ? AND user_id = ?", id, user.ID).Scan(&date); scanErr != nil {
		log.Printf("UpdateEntry: failed to get date for entry %d: %v", id, scanErr)
	}

	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	http.Redirect(w, r, "/?date="+date, http.StatusSeeOther)
}
