package handlers

import (
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/mpdroog/mycal/db"
	"github.com/mpdroog/mycal/models"
)

type foodFormData struct {
	Name        string
	Calories    int
	Protein     float64
	Carbs       float64
	Fat         float64
	ServingSize string
}

func parseFoodForm(r *http.Request) (foodFormData, error) {
	if err := r.ParseForm(); err != nil {
		return foodFormData{}, err
	}

	calories, err := strconv.Atoi(r.FormValue("calories"))
	if err != nil {
		return foodFormData{}, fmt.Errorf("invalid calories: %w", err)
	}

	protein, err := strconv.ParseFloat(r.FormValue("protein"), 64)
	if err != nil {
		return foodFormData{}, fmt.Errorf("invalid protein: %w", err)
	}

	carbs, err := strconv.ParseFloat(r.FormValue("carbs"), 64)
	if err != nil {
		return foodFormData{}, fmt.Errorf("invalid carbs: %w", err)
	}

	fat, err := strconv.ParseFloat(r.FormValue("fat"), 64)
	if err != nil {
		return foodFormData{}, fmt.Errorf("invalid fat: %w", err)
	}

	return foodFormData{
		Name:        r.FormValue("name"),
		Calories:    calories,
		Protein:     protein,
		Carbs:       carbs,
		Fat:         fat,
		ServingSize: r.FormValue("serving_size"),
	}, nil
}

func ListFoods(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.DB.Query(`
			SELECT id, name, calories, protein, carbs, fat, serving_size, created_at
			FROM foods ORDER BY name ASC
		`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}
		defer rows.Close()

		var foods []models.Food

		for rows.Next() {
			var f models.Food

			err := rows.Scan(&f.ID, &f.Name, &f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.ServingSize, &f.CreatedAt)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}

			foods = append(foods, f)
		}

		if err := rows.Err(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		data := map[string]interface{}{
			"Title": "Foods",
			"Foods": foods,
		}

		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func CreateFood(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			data := map[string]interface{}{
				"Title": "Add Food",
				"Food":  models.Food{ServingSize: "1 serving"},
			}

			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

			return
		}

		form, err := parseFoodForm(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		_, err = db.DB.Exec(`
			INSERT INTO foods (name, calories, protein, carbs, fat, serving_size)
			VALUES (?, ?, ?, ?, ?, ?)
		`, form.Name, form.Calories, form.Protein, form.Carbs, form.Fat, form.ServingSize)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		http.Redirect(w, r, "/foods", http.StatusSeeOther)
	}
}

func showEditFoodForm(tmpl *template.Template, w http.ResponseWriter, r *http.Request, id int64) {
	var f models.Food

	queryErr := db.DB.QueryRow(`
		SELECT id, name, calories, protein, carbs, fat, serving_size, created_at
		FROM foods WHERE id = ?
	`, id).Scan(&f.ID, &f.Name, &f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.ServingSize, &f.CreatedAt)
	if errors.Is(queryErr, models.ErrNotFound) {
		http.NotFound(w, r)

		return
	}

	if queryErr != nil {
		http.Error(w, queryErr.Error(), http.StatusInternalServerError)

		return
	}

	data := map[string]interface{}{
		"Title": "Edit Food",
		"Food":  f,
	}

	if tmplErr := tmpl.ExecuteTemplate(w, "base", data); tmplErr != nil {
		http.Error(w, tmplErr.Error(), http.StatusInternalServerError)
	}
}

func EditFood(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)

			return
		}

		if r.Method == http.MethodGet {
			showEditFoodForm(tmpl, w, r, id)

			return
		}

		form, err := parseFoodForm(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		_, err = db.DB.Exec(`
			UPDATE foods SET name = ?, calories = ?, protein = ?, carbs = ?, fat = ?, serving_size = ?
			WHERE id = ?
		`, form.Name, form.Calories, form.Protein, form.Carbs, form.Fat, form.ServingSize, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		http.Redirect(w, r, "/foods", http.StatusSeeOther)
	}
}

func DeleteFood(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)

		return
	}

	_, err = db.DB.Exec("DELETE FROM foods WHERE id = ?", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/foods", http.StatusSeeOther)
}

func SearchFoods(w http.ResponseWriter, r *http.Request) {
	q := "%" + r.URL.Query().Get("q") + "%"

	rows, err := db.DB.Query(`
		SELECT id, name, calories, protein, carbs, fat, serving_size
		FROM foods WHERE name LIKE ? ORDER BY name LIMIT 10
	`, q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/html")

	for rows.Next() {
		var f models.Food

		if err := rows.Scan(&f.ID, &f.Name, &f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.ServingSize); err != nil {
			log.Printf("SearchFoods scan error: %v", err)

			continue
		}

		// Return as HTMX-friendly options
		option := fmt.Sprintf(
			`<option value="%d" data-calories="%d">%s (%d cal)</option>`,
			f.ID, f.Calories, f.Name, f.Calories,
		)

		if _, err := w.Write([]byte(option)); err != nil {
			log.Printf("SearchFoods write error: %v", err)

			return
		}
	}

	if err := rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
