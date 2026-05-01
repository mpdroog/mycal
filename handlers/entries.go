package handlers

import (
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mpdroog/mycal/db"
	"github.com/mpdroog/mycal/models"
)

func Dashboard(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		date := r.URL.Query().Get("date")
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}

		summary := getDaySummary(date)

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
		profile, err := GetProfile()
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
		}

		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func CreateEntry(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	var foodID int64

	// Check if ingredient_id is provided (quick add from ingredient)
	if ingredientIDStr := r.FormValue("ingredient_id"); ingredientIDStr != "" {
		ingredientID, err := strconv.ParseInt(ingredientIDStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid ingredient_id", http.StatusBadRequest)

			return
		}

		// Get ingredient name
		var ingredientName string

		err = db.DB.QueryRow("SELECT name FROM ingredients WHERE id = ?", ingredientID).Scan(&ingredientName)
		if err != nil {
			http.Error(w, "ingredient not found", http.StatusBadRequest)

			return
		}

		// Check if a simple food already exists for this ingredient
		err = db.DB.QueryRow(`
			SELECT f.id FROM foods f
			JOIN food_ingredients fi ON f.id = fi.food_id
			WHERE f.name = ? AND fi.ingredient_id = ? AND fi.amount_grams = 100
			GROUP BY f.id HAVING COUNT(*) = 1
		`, ingredientName, ingredientID).Scan(&foodID)

		if err != nil {
			// Create a simple food from this ingredient (100g)
			result, err := db.DB.Exec("INSERT INTO foods (name) VALUES (?)", ingredientName)
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

	servings, err := strconv.ParseFloat(r.FormValue("servings"), 64)
	if err != nil {
		http.Error(w, "invalid servings value", http.StatusBadRequest)

		return
	}

	if servings <= 0 {
		servings = 1
	}

	date := r.FormValue("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	_, err = db.DB.Exec(`
		INSERT INTO entries (food_id, date, meal, servings, notes)
		VALUES (?, ?, ?, ?, ?)
	`, foodID, date, r.FormValue("meal"), servings, r.FormValue("notes"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/?date="+date, http.StatusSeeOther)
}

func DeleteEntry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)

		return
	}

	// Get the date before deleting
	var date string

	if scanErr := db.DB.QueryRow("SELECT date FROM entries WHERE id = ?", id).Scan(&date); scanErr != nil {
		log.Printf("DeleteEntry: failed to get date for entry %d: %v", id, scanErr)
	}

	_, err = db.DB.Exec("DELETE FROM entries WHERE id = ?", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	http.Redirect(w, r, "/?date="+date, http.StatusSeeOther)
}

func getDaySummary(date string) models.DaySummary {
	summary := models.DaySummary{Date: date}

	rows, err := db.DB.Query(`
		SELECT e.id, e.food_id, e.date, e.meal, e.servings, COALESCE(e.notes, ''),
		       f.id, f.name
		FROM entries e
		JOIN foods f ON e.food_id = f.id
		WHERE e.date = ?
		ORDER BY e.created_at ASC
	`, date)
	if err != nil {
		log.Printf("getDaySummary: query failed: %v", err)

		return summary
	}
	defer rows.Close()

	for rows.Next() {
		var e models.Entry
		var f models.Food

		if err := rows.Scan(&e.ID, &e.FoodID, &e.Date, &e.Meal, &e.Servings, &e.Notes,
			&f.ID, &f.Name); err != nil {
			log.Printf("getDaySummary: scan failed: %v", err)

			continue
		}

		// Get nutrition for this food
		foodWithNutrition, err := getFoodWithNutrition(f.ID)
		if err != nil {
			log.Printf("getDaySummary: failed to get nutrition for food %d: %v", f.ID, err)
			e.Food = &f
		} else {
			e.Food = foodWithNutrition
		}

		// Calculate totals based on servings
		if e.Food != nil {
			summary.Calories += int(float64(e.Food.Calories) * e.Servings)
			summary.Protein += e.Food.Protein * e.Servings
			summary.Carbs += e.Food.Carbs * e.Servings
			summary.Fat += e.Food.Fat * e.Servings
		}

		summary.Entries = append(summary.Entries, e)
	}

	if err := rows.Err(); err != nil {
		log.Printf("getDaySummary: rows error: %v", err)
	}

	return summary
}

func GetEntry(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
			WHERE e.id = ?
		`, id).Scan(&e.ID, &e.FoodID, &e.Date, &e.Meal, &e.Servings, &e.Notes,
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
		}

		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func UpdateEntryServings(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)

		return
	}

	if parseErr := r.ParseForm(); parseErr != nil {
		http.Error(w, parseErr.Error(), http.StatusBadRequest)

		return
	}

	servings, err := strconv.ParseFloat(r.FormValue("servings"), 64)
	if err != nil {
		http.Error(w, "invalid servings value", http.StatusBadRequest)

		return
	}

	if servings <= 0 {
		servings = 0.25
	}

	_, err = db.DB.Exec(`UPDATE entries SET servings = ? WHERE id = ?`, servings, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
}

func UpdateEntry(w http.ResponseWriter, r *http.Request) {
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

	_, err = db.DB.Exec(`
		UPDATE entries SET food_id = ?, meal = ?, servings = ?, notes = ?
		WHERE id = ?
	`, foodID, r.FormValue("meal"), servings, r.FormValue("notes"), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	// Get date for redirect
	var date string

	if scanErr := db.DB.QueryRow("SELECT date FROM entries WHERE id = ?", id).Scan(&date); scanErr != nil {
		log.Printf("UpdateEntry: failed to get date for entry %d: %v", id, scanErr)
	}

	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	http.Redirect(w, r, "/?date="+date, http.StatusSeeOther)
}
