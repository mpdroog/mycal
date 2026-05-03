package handlers

import (
	"encoding/csv"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/mpdroog/mycal/auth"
	"github.com/mpdroog/mycal/models"
	"github.com/mpdroog/mycal/safehtml"
)

type ingredientFormData struct {
	Name        string
	Calories    int
	Protein     float64
	Carbs       float64
	Fat         float64
	ServingSize string
	ServingType string
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

	servingType := r.FormValue("serving_type")
	if servingType == "" {
		servingType = "weight"
	}

	servingSize := r.FormValue("serving_size")
	if servingType == "weight" {
		servingSize = "100g"
	}

	return ingredientFormData{
		Name:        r.FormValue("name"),
		Calories:    calories,
		Protein:     protein,
		Carbs:       carbs,
		Fat:         fat,
		ServingSize: servingSize,
		ServingType: servingType,
	}, nil
}

const itemsPerPage = 20

func ListIngredients(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := auth.GetUserFromContext(r.Context())

		query := r.URL.Query().Get("q")

		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil || page < 1 {
			page = 1
		}
		offset := (page - 1) * itemsPerPage

		ingredients, totalCount, err := models.ListIngredients(query, itemsPerPage, offset)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}

		totalPages := (totalCount + itemsPerPage - 1) / itemsPerPage
		if totalPages < 1 {
			totalPages = 1
		}

		data := map[string]interface{}{
			"Title":       "Ingredients",
			"Ingredients": ingredients,
			"User":        user,
			"Query":       query,
			"Page":        page,
			"TotalPages":  totalPages,
			"TotalCount":  totalCount,
			"HasPrev":     page > 1,
			"HasNext":     page < totalPages,
			"PrevPage":    page - 1,
			"NextPage":    page + 1,
		}

		if imported := r.URL.Query().Get("imported"); imported != "" {
			data["Imported"] = imported
		}
		if skipped := r.URL.Query().Get("skipped"); skipped != "" {
			data["Skipped"] = skipped
		}

		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			httpError(w, err, http.StatusInternalServerError)
		}
	}
}

func CreateIngredient(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := auth.GetUserFromContext(r.Context())

		if r.Method == http.MethodGet {
			data := map[string]interface{}{
				"Title":      "Add Ingredient",
				"Ingredient": models.Ingredient{ServingSize: "100g", ServingType: "weight"},
				"User":       user,
			}
			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				httpError(w, err, http.StatusInternalServerError)
			}
			return
		}

		form, err := parseIngredientForm(r)
		if err != nil {
			httpError(w, err, http.StatusBadRequest)
			return
		}

		ingredient := &models.Ingredient{
			Name:        form.Name,
			Calories:    form.Calories,
			Protein:     form.Protein,
			Carbs:       form.Carbs,
			Fat:         form.Fat,
			ServingSize: form.ServingSize,
			ServingType: form.ServingType,
		}

		id, err := models.CreateIngredient(ingredient)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}

		if err := models.IndexItem("ingredient", id, form.Name); err != nil {
			log.Printf("CreateIngredient: failed to index: %v", err)
		}

		http.Redirect(w, r, "/ingredients", http.StatusSeeOther)
	}
}

func EditIngredient(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := auth.GetUserFromContext(r.Context())

		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		if r.Method == http.MethodGet {
			ingredient, getErr := models.GetIngredient(id)
			if errors.Is(getErr, models.ErrNotFound) {
				http.NotFound(w, r)
				return
			}

			if getErr != nil {
				httpError(w, getErr, http.StatusInternalServerError)
				return
			}

			data := map[string]interface{}{
				"Title":      "Edit Ingredient",
				"Ingredient": ingredient,
				"User":       user,
			}
			renderTemplate(w, tmpl, data)
			return
		}

		form, err := parseIngredientForm(r)
		if err != nil {
			httpError(w, err, http.StatusBadRequest)
			return
		}

		ingredient := &models.Ingredient{
			ID:          id,
			Name:        form.Name,
			Calories:    form.Calories,
			Protein:     form.Protein,
			Carbs:       form.Carbs,
			Fat:         form.Fat,
			ServingSize: form.ServingSize,
			ServingType: form.ServingType,
		}

		if err := models.UpdateIngredient(ingredient); err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}

		if err := models.IndexItem("ingredient", id, form.Name); err != nil {
			log.Printf("EditIngredient: failed to index: %v", err)
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

	if err := models.DeleteIngredient(id); err != nil {
		if errors.Is(err, models.ErrInUse) {
			http.Error(w, "Cannot delete ingredient: it is used in foods. Remove it from foods first.", http.StatusConflict)
			return
		}
		httpError(w, err, http.StatusInternalServerError)
		return
	}

	if err := models.RemoveItemIndex("ingredient", id); err != nil {
		log.Printf("DeleteIngredient: failed to remove index: %v", err)
	}

	http.Redirect(w, r, "/ingredients?deleted=ingredient&id="+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func RestoreIngredient(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	name, err := models.RestoreIngredient(id)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}

	if name != "" {
		if err := models.IndexItem("ingredient", id, name); err != nil {
			log.Printf("RestoreIngredient: failed to index: %v", err)
		}
	}

	http.Redirect(w, r, "/ingredients", http.StatusSeeOther)
}

func SearchIngredients(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	results, err := models.SearchIngredients("%"+q+"%", 10)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	for _, i := range results {
		option := safehtml.Sprintf(
			`<option value="%d" data-calories="%d" data-protein="%.1f" data-carbs="%.1f" data-fat="%.1f" data-serving-type="%s" data-serving-size="%s">%s (%d kcal/%s)</option>`,
			i.ID, i.Calories, i.Protein, i.Carbs, i.Fat, i.ServingType, i.ServingSize, i.Name, i.Calories, i.ServingSize,
		)
		if _, err := w.Write([]byte(option)); err != nil {
			log.Printf("SearchIngredients write error: %v", err)
			return
		}
	}
}

func ImportIngredients(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("csv")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)

	header, err := reader.Read()
	if err != nil {
		httpError(w, fmt.Errorf("invalid CSV header: %w", err), http.StatusBadRequest)
		return
	}

	colMap := make(map[string]int)
	for i, col := range header {
		colMap[col] = i
	}

	required := []string{"name", "calories", "protein", "carbs", "fat"}
	for _, col := range required {
		if _, ok := colMap[col]; !ok {
			http.Error(w, "Missing required column: "+col, http.StatusBadRequest)
			return
		}
	}

	var imported, skipped int

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			log.Printf("ImportIngredients: error reading row: %v", err)
			skipped++
			continue
		}

		name := record[colMap["name"]]
		if name == "" {
			skipped++
			continue
		}

		calories, err := strconv.Atoi(record[colMap["calories"]])
		if err != nil {
			log.Printf("ImportIngredients: invalid calories for %s: %v", name, err)
			skipped++
			continue
		}

		protein, err := strconv.ParseFloat(record[colMap["protein"]], 64)
		if err != nil {
			log.Printf("ImportIngredients: invalid protein for %s: %v", name, err)
			skipped++
			continue
		}

		carbs, err := strconv.ParseFloat(record[colMap["carbs"]], 64)
		if err != nil {
			log.Printf("ImportIngredients: invalid carbs for %s: %v", name, err)
			skipped++
			continue
		}

		fat, err := strconv.ParseFloat(record[colMap["fat"]], 64)
		if err != nil {
			log.Printf("ImportIngredients: invalid fat for %s: %v", name, err)
			skipped++
			continue
		}

		servingSize := "100g"
		servingType := "weight"
		if idx, ok := colMap["serving_size"]; ok && idx < len(record) && record[idx] != "" {
			servingSize = record[idx]
		}
		if idx, ok := colMap["serving_type"]; ok && idx < len(record) && record[idx] != "" {
			servingType = record[idx]
		}

		if _, exists := models.IngredientExists(name); exists {
			skipped++
			continue
		}

		ingredient := &models.Ingredient{
			Name:        name,
			Calories:    calories,
			Protein:     protein,
			Carbs:       carbs,
			Fat:         fat,
			ServingSize: servingSize,
			ServingType: servingType,
		}

		id, err := models.CreateIngredient(ingredient)
		if err != nil {
			log.Printf("ImportIngredients: error inserting %s: %v", name, err)
			skipped++
			continue
		}

		if err := models.IndexItem("ingredient", id, name); err != nil {
			log.Printf("ImportIngredients: failed to index %s: %v", name, err)
		}

		imported++
	}

	http.Redirect(w, r, fmt.Sprintf("/ingredients?imported=%d&skipped=%d", imported, skipped), http.StatusSeeOther)
}
