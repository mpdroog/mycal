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

type ingredientFormData struct {
	Name        string
	Calories    int
	Protein     float64
	Carbs       float64
	Fat         float64
	ServingSize string
}

func parseIngredientForm(r *http.Request) (ingredientFormData, error) {
	if err := r.ParseForm(); err != nil {
		return ingredientFormData{}, err
	}

	calories, err := strconv.Atoi(r.FormValue("calories"))
	if err != nil {
		return ingredientFormData{}, fmt.Errorf("invalid calories: %w", err)
	}

	protein, err := strconv.ParseFloat(r.FormValue("protein"), 64)
	if err != nil {
		return ingredientFormData{}, fmt.Errorf("invalid protein: %w", err)
	}

	carbs, err := strconv.ParseFloat(r.FormValue("carbs"), 64)
	if err != nil {
		return ingredientFormData{}, fmt.Errorf("invalid carbs: %w", err)
	}

	fat, err := strconv.ParseFloat(r.FormValue("fat"), 64)
	if err != nil {
		return ingredientFormData{}, fmt.Errorf("invalid fat: %w", err)
	}

	return ingredientFormData{
		Name:        r.FormValue("name"),
		Calories:    calories,
		Protein:     protein,
		Carbs:       carbs,
		Fat:         fat,
		ServingSize: r.FormValue("serving_size"),
	}, nil
}

func ListIngredients(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.DB.Query(`
			SELECT id, name, calories, protein, carbs, fat, serving_size, created_at
			FROM ingredients ORDER BY name ASC
		`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}
		defer rows.Close()

		var ingredients []models.Ingredient

		for rows.Next() {
			var i models.Ingredient

			err := rows.Scan(&i.ID, &i.Name, &i.Calories, &i.Protein, &i.Carbs, &i.Fat, &i.ServingSize, &i.CreatedAt)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}

			ingredients = append(ingredients, i)
		}

		if err := rows.Err(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		data := map[string]interface{}{
			"Title":       "Ingredients",
			"Ingredients": ingredients,
		}

		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func CreateIngredient(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			data := map[string]interface{}{
				"Title":      "Add Ingredient",
				"Ingredient": models.Ingredient{ServingSize: "100g"},
			}

			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

			return
		}

		form, err := parseIngredientForm(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		_, err = db.DB.Exec(`
			INSERT INTO ingredients (name, calories, protein, carbs, fat, serving_size)
			VALUES (?, ?, ?, ?, ?, ?)
		`, form.Name, form.Calories, form.Protein, form.Carbs, form.Fat, form.ServingSize)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		http.Redirect(w, r, "/ingredients", http.StatusSeeOther)
	}
}

func showEditIngredientForm(tmpl *template.Template, w http.ResponseWriter, r *http.Request, id int64) {
	var i models.Ingredient

	queryErr := db.DB.QueryRow(`
		SELECT id, name, calories, protein, carbs, fat, serving_size, created_at
		FROM ingredients WHERE id = ?
	`, id).Scan(&i.ID, &i.Name, &i.Calories, &i.Protein, &i.Carbs, &i.Fat, &i.ServingSize, &i.CreatedAt)
	if errors.Is(queryErr, models.ErrNotFound) {
		http.NotFound(w, r)

		return
	}

	if queryErr != nil {
		http.Error(w, queryErr.Error(), http.StatusInternalServerError)

		return
	}

	data := map[string]interface{}{
		"Title":      "Edit Ingredient",
		"Ingredient": i,
	}

	if tmplErr := tmpl.ExecuteTemplate(w, "base", data); tmplErr != nil {
		http.Error(w, tmplErr.Error(), http.StatusInternalServerError)
	}
}

func EditIngredient(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)

			return
		}

		if r.Method == http.MethodGet {
			showEditIngredientForm(tmpl, w, r, id)

			return
		}

		form, err := parseIngredientForm(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		_, err = db.DB.Exec(`
			UPDATE ingredients SET name = ?, calories = ?, protein = ?, carbs = ?, fat = ?, serving_size = ?
			WHERE id = ?
		`, form.Name, form.Calories, form.Protein, form.Carbs, form.Fat, form.ServingSize, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		http.Redirect(w, r, "/ingredients", http.StatusSeeOther)
	}
}

func DeleteIngredient(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)

		return
	}

	// Check if this ingredient is used in any foods
	var usageCount int

	err = db.DB.QueryRow("SELECT COUNT(*) FROM food_ingredients WHERE ingredient_id = ?", id).Scan(&usageCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	if usageCount > 0 {
		http.Error(w, "Cannot delete ingredient: it is used in foods. Remove it from foods first.", http.StatusConflict)

		return
	}

	_, err = db.DB.Exec("DELETE FROM ingredients WHERE id = ?", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/ingredients", http.StatusSeeOther)
}

func SearchIngredients(w http.ResponseWriter, r *http.Request) {
	q := "%" + r.URL.Query().Get("q") + "%"

	rows, err := db.DB.Query(`
		SELECT id, name, calories, protein, carbs, fat, serving_size
		FROM ingredients WHERE name LIKE ? ORDER BY name LIMIT 10
	`, q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/html")

	for rows.Next() {
		var i models.Ingredient

		if err := rows.Scan(&i.ID, &i.Name, &i.Calories, &i.Protein, &i.Carbs, &i.Fat, &i.ServingSize); err != nil {
			log.Printf("SearchIngredients scan error: %v", err)

			continue
		}

		option := fmt.Sprintf(
			`<option value="%d" data-calories="%d" data-protein="%.1f" data-carbs="%.1f" data-fat="%.1f">%s (%d kcal/%s)</option>`,
			i.ID, i.Calories, i.Protein, i.Carbs, i.Fat, i.Name, i.Calories, i.ServingSize,
		)

		if _, err := w.Write([]byte(option)); err != nil {
			log.Printf("SearchIngredients write error: %v", err)

			return
		}
	}

	if err := rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// GetAllIngredients returns all ingredients for use in other handlers.
func GetAllIngredients() ([]models.Ingredient, error) {
	rows, err := db.DB.Query(`SELECT id, name, calories, protein, carbs, fat, serving_size FROM ingredients ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ingredients []models.Ingredient

	for rows.Next() {
		var i models.Ingredient

		if err := rows.Scan(&i.ID, &i.Name, &i.Calories, &i.Protein, &i.Carbs, &i.Fat, &i.ServingSize); err != nil {
			return nil, err
		}

		ingredients = append(ingredients, i)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ingredients, nil
}
