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
	"github.com/mpdroog/mycal/models"
)

func ListFoods(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := auth.GetUserFromContext(r.Context())

		query := r.URL.Query().Get("q")
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		offset := (page - 1) * itemsPerPage

		foods, totalCount, err := models.ListFoods(query, itemsPerPage, offset)
		if err != nil {
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

		var foodIngredients []models.FoodIngredient
		ingredientsJSON := r.FormValue("ingredients")
		if ingredientsJSON != "" {
			var parsed []struct {
				IngredientID int64   `json:"ingredient_id"`
				AmountGrams  float64 `json:"amount_grams"`
			}
			if err := json.Unmarshal([]byte(ingredientsJSON), &parsed); err != nil {
				http.Error(w, "invalid ingredients format", http.StatusBadRequest)
				return
			}
			for _, p := range parsed {
				foodIngredients = append(foodIngredients, models.FoodIngredient{
					IngredientID: p.IngredientID,
					AmountGrams:  p.AmountGrams,
				})
			}
		}

		foodID, err := models.CreateFood(name, foodIngredients)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}

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
			food, err := models.GetFood(id)
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

		var foodIngredients []models.FoodIngredient
		ingredientsJSON := r.FormValue("ingredients")
		if ingredientsJSON != "" {
			var parsed []struct {
				IngredientID int64   `json:"ingredient_id"`
				AmountGrams  float64 `json:"amount_grams"`
			}
			if err := json.Unmarshal([]byte(ingredientsJSON), &parsed); err != nil {
				http.Error(w, "invalid ingredients format", http.StatusBadRequest)
				return
			}
			for _, p := range parsed {
				foodIngredients = append(foodIngredients, models.FoodIngredient{
					IngredientID: p.IngredientID,
					AmountGrams:  p.AmountGrams,
				})
			}
		}

		if err := models.UpdateFood(id, name, foodIngredients); err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}

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

	if err := models.DeleteFood(id); err != nil {
		if errors.Is(err, models.ErrInUse) {
			http.Error(w, "Cannot delete food: it is used in diary entries. Delete the entries first.", http.StatusConflict)
			return
		}
		httpError(w, err, http.StatusInternalServerError)
		return
	}

	_ = models.RemoveItemIndex("food", id)
	http.Redirect(w, r, "/foods?deleted=food&id="+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func RestoreFood(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	name, err := models.RestoreFood(id)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}

	if name != "" {
		_ = models.IndexItem("food", id, name)
	}
	http.Redirect(w, r, "/foods", http.StatusSeeOther)
}

func SearchFoods(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	results, err := models.SearchFoods("%"+q+"%", 10)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if results == nil {
		results = []models.SearchItem{}
	}
	if err := json.NewEncoder(w).Encode(results); err != nil {
		log.Printf("SearchFoods encode error: %v", err)
	}
}
