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

		data := map[string]interface{}{
			"Title":   "Today",
			"Date":    date,
			"Summary": summary,
			"Foods":   foods,
			"Meals":   []string{"breakfast", "lunch", "dinner", "snack"},
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
		       f.id, f.name, f.calories, f.protein, f.carbs, f.fat, f.serving_size
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
			&f.ID, &f.Name, &f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.ServingSize); err != nil {
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

func getAllFoods() ([]models.Food, error) {
	rows, err := db.DB.Query(`SELECT id, name, calories, serving_size FROM foods ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var foods []models.Food

	for rows.Next() {
		var f models.Food

		if err := rows.Scan(&f.ID, &f.Name, &f.Calories, &f.ServingSize); err != nil {
			return nil, err
		}

		foods = append(foods, f)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return foods, nil
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
			       f.id, f.name, f.calories, f.protein, f.carbs, f.fat, f.serving_size
			FROM entries e
			JOIN foods f ON e.food_id = f.id
			WHERE e.id = ?
		`, id).Scan(&e.ID, &e.FoodID, &e.Date, &e.Meal, &e.Servings, &e.Notes,
			&f.ID, &f.Name, &f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.ServingSize)
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
