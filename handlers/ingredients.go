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
	"github.com/mpdroog/mycal/db"
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

		// Get search and pagination params
		query := r.URL.Query().Get("q")
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		offset := (page - 1) * itemsPerPage

		// Build query with optional search
		var totalCount int
		var countQuery, listQuery string
		var countArgs, listArgs []interface{}

		if query != "" {
			searchPattern := "%" + query + "%"
			countQuery = `SELECT COUNT(*) FROM ingredients WHERE deleted_at IS NULL AND name LIKE ?`
			countArgs = []interface{}{searchPattern}
			listQuery = `SELECT id, name, calories, protein, carbs, fat, serving_size, serving_type, created_at
				FROM ingredients WHERE deleted_at IS NULL AND name LIKE ? ORDER BY name ASC LIMIT ? OFFSET ?`
			listArgs = []interface{}{searchPattern, itemsPerPage, offset}
		} else {
			countQuery = `SELECT COUNT(*) FROM ingredients WHERE deleted_at IS NULL`
			countArgs = nil
			listQuery = `SELECT id, name, calories, protein, carbs, fat, serving_size, serving_type, created_at
				FROM ingredients WHERE deleted_at IS NULL ORDER BY name ASC LIMIT ? OFFSET ?`
			listArgs = []interface{}{itemsPerPage, offset}
		}

		if err := db.DB.QueryRow(countQuery, countArgs...).Scan(&totalCount); err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}

		rows, err := db.DB.Query(listQuery, listArgs...)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)

			return
		}
		defer rows.Close()

		var ingredients []models.Ingredient

		for rows.Next() {
			var i models.Ingredient

			err := rows.Scan(&i.ID, &i.Name, &i.Calories, &i.Protein, &i.Carbs, &i.Fat, &i.ServingSize, &i.ServingType, &i.CreatedAt)
			if err != nil {
				httpError(w, err, http.StatusInternalServerError)

				return
			}

			ingredients = append(ingredients, i)
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

		// Pass import results if present
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

		result, err := db.DB.Exec(`
			INSERT INTO ingredients (name, calories, protein, carbs, fat, serving_size, serving_type)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, form.Name, form.Calories, form.Protein, form.Carbs, form.Fat, form.ServingSize, form.ServingType)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)

			return
		}

		// Index for fuzzy search
		if id, err := result.LastInsertId(); err == nil {
			_ = models.IndexItem("ingredient", id, form.Name)
		}

		http.Redirect(w, r, "/ingredients", http.StatusSeeOther)
	}
}

func showEditIngredientForm(tmpl *template.Template, w http.ResponseWriter, r *http.Request, id int64, user *models.User) {
	var i models.Ingredient

	queryErr := db.DB.QueryRow(`
		SELECT id, name, calories, protein, carbs, fat, serving_size, serving_type, created_at
		FROM ingredients WHERE id = ?
	`, id).Scan(&i.ID, &i.Name, &i.Calories, &i.Protein, &i.Carbs, &i.Fat, &i.ServingSize, &i.ServingType, &i.CreatedAt)
	if errors.Is(queryErr, models.ErrNotFound) {
		http.NotFound(w, r)

		return
	}

	if queryErr != nil {
		httpError(w, queryErr, http.StatusInternalServerError)

		return
	}

	data := map[string]interface{}{
		"Title":      "Edit Ingredient",
		"Ingredient": i,
		"User":       user,
	}

	if tmplErr := tmpl.ExecuteTemplate(w, "base", data); tmplErr != nil {
		httpError(w, tmplErr, http.StatusInternalServerError)
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
			showEditIngredientForm(tmpl, w, r, id, user)

			return
		}

		form, err := parseIngredientForm(r)
		if err != nil {
			httpError(w, err, http.StatusBadRequest)

			return
		}

		_, err = db.DB.Exec(`
			UPDATE ingredients SET name = ?, calories = ?, protein = ?, carbs = ?, fat = ?, serving_size = ?, serving_type = ?
			WHERE id = ?
		`, form.Name, form.Calories, form.Protein, form.Carbs, form.Fat, form.ServingSize, form.ServingType, id)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)

			return
		}

		// Re-index for fuzzy search
		_ = models.IndexItem("ingredient", id, form.Name)

		http.Redirect(w, r, "/ingredients", http.StatusSeeOther)
	}
}

func DeleteIngredient(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)

		return
	}

	// Check if this ingredient is used in any non-deleted foods
	var usageCount int

	err = db.DB.QueryRow(`
		SELECT COUNT(*) FROM food_ingredients fi
		JOIN foods f ON fi.food_id = f.id
		WHERE fi.ingredient_id = ? AND f.deleted_at IS NULL
	`, id).Scan(&usageCount)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)

		return
	}

	if usageCount > 0 {
		http.Error(w, "Cannot delete ingredient: it is used in foods. Remove it from foods first.", http.StatusConflict)

		return
	}

	// Soft delete: set deleted_at timestamp
	_, err = db.DB.Exec("UPDATE ingredients SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)

		return
	}

	// Remove from search index
	_ = models.RemoveItemIndex("ingredient", id)

	http.Redirect(w, r, "/ingredients?deleted=ingredient&id="+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func RestoreIngredient(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)

		return
	}

	// Get name for re-indexing
	var name string
	_ = db.DB.QueryRow("SELECT name FROM ingredients WHERE id = ?", id).Scan(&name)

	// Clear deleted_at to restore
	_, err = db.DB.Exec("UPDATE ingredients SET deleted_at = NULL WHERE id = ?", id)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)

		return
	}

	// Re-index for fuzzy search
	if name != "" {
		_ = models.IndexItem("ingredient", id, name)
	}

	http.Redirect(w, r, "/ingredients", http.StatusSeeOther)
}

func SearchIngredients(w http.ResponseWriter, r *http.Request) {
	q := "%" + r.URL.Query().Get("q") + "%"

	rows, err := db.DB.Query(`
		SELECT id, name, calories, protein, carbs, fat, serving_size, serving_type
		FROM ingredients WHERE name LIKE ? AND deleted_at IS NULL ORDER BY name LIMIT 10
	`, q)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)

		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/html")

	for rows.Next() {
		var i models.Ingredient

		if err := rows.Scan(&i.ID, &i.Name, &i.Calories, &i.Protein, &i.Carbs, &i.Fat, &i.ServingSize, &i.ServingType); err != nil {
			log.Printf("SearchIngredients scan error: %v", err)

			continue
		}

		option := safehtml.Sprintf(
			`<option value="%d" data-calories="%d" data-protein="%.1f" data-carbs="%.1f" data-fat="%.1f" data-serving-type="%s" data-serving-size="%s">%s (%d kcal/%s)</option>`,
			i.ID, i.Calories, i.Protein, i.Carbs, i.Fat, i.ServingType, i.ServingSize, i.Name, i.Calories, i.ServingSize,
		)

		if _, err := w.Write([]byte(option)); err != nil {
			log.Printf("SearchIngredients write error: %v", err)

			return
		}
	}

	if err := rows.Err(); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}

// GetAllIngredients returns all ingredients for use in other handlers.
func GetAllIngredients() ([]models.Ingredient, error) {
	rows, err := db.DB.Query(`SELECT id, name, calories, protein, carbs, fat, serving_size, serving_type FROM ingredients WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ingredients []models.Ingredient

	for rows.Next() {
		var i models.Ingredient

		if err := rows.Scan(&i.ID, &i.Name, &i.Calories, &i.Protein, &i.Carbs, &i.Fat, &i.ServingSize, &i.ServingType); err != nil {
			return nil, err
		}

		ingredients = append(ingredients, i)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ingredients, nil
}

// ImportIngredients handles CSV file upload and imports ingredients.
// CSV format: name,calories,protein,carbs,fat,serving_size
func ImportIngredients(w http.ResponseWriter, r *http.Request) {
	// Max 10MB file
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

	// Read header
	header, err := reader.Read()
	if err != nil {
		httpError(w, fmt.Errorf("invalid CSV header: %w", err), http.StatusBadRequest)

		return
	}

	// Map column names to indices
	colMap := make(map[string]int)
	for i, col := range header {
		colMap[col] = i
	}

	// Check required columns
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
		if err == io.EOF {
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

		// Check if ingredient already exists
		var existingID int64

		existErr := db.DB.QueryRow("SELECT id FROM ingredients WHERE name = ?", name).Scan(&existingID)
		if existErr == nil {
			// Already exists, skip
			skipped++

			continue
		}

		// Insert new ingredient
		result, insertErr := db.DB.Exec(`
			INSERT INTO ingredients (name, calories, protein, carbs, fat, serving_size, serving_type)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, name, calories, protein, carbs, fat, servingSize, servingType)
		if insertErr != nil {
			log.Printf("ImportIngredients: error inserting %s: %v", name, insertErr)
			skipped++

			continue
		}

		// Index for fuzzy search
		if id, err := result.LastInsertId(); err == nil {
			_ = models.IndexItem("ingredient", id, name)
		}

		imported++
	}

	http.Redirect(w, r, fmt.Sprintf("/ingredients?imported=%d&skipped=%d", imported, skipped), http.StatusSeeOther)
}
