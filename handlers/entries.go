package handlers

import (
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mpdroog/mycal/auth"
	"github.com/mpdroog/mycal/models"
)

func Dashboard(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := auth.GetUserFromContext(r.Context())

		date := r.URL.Query().Get("date")
		if date == "" {
			date = time.Now().Format("2006-01-02")
		} else {
			date = normalizeDate(date)
		}

		summary := models.GetDaySummary(date, user.ID)
		profile := models.GetProfile(user.ID)

		data := map[string]interface{}{
			"Title":    "Today",
			"Date":     date,
			"Summary":  summary,
			"Meals":    []string{"breakfast", "lunch", "dinner", "snack"},
			"Profile":  profile,
			"User":     user,
			"HasItems": models.HasIngredients(),
		}

		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			httpError(w, err, http.StatusInternalServerError)
		}
	}
}

func CreateEntry(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}

	var foodID int64
	servingType := "weight"

	// Check if ingredient_id is provided (quick add from ingredient)
	if ingredientIDStr := r.FormValue("ingredient_id"); ingredientIDStr != "" {
		ingredientID, err := strconv.ParseInt(ingredientIDStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid ingredient_id", http.StatusBadRequest)
			return
		}

		foodID, servingType, err = models.FindOrCreateSimpleFood(ingredientID)
		if err != nil {
			http.Error(w, "ingredient not found", http.StatusBadRequest)
			return
		}
	} else {
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
		servingsInput = 100
		if servingType == "unit" {
			servingsInput = 1
		}
	}

	var servings float64
	if servingType == "weight" {
		servings = servingsInput / 100
	} else {
		servings = servingsInput
	}

	date := r.FormValue("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	entry := &models.Entry{
		UserID:   user.ID,
		FoodID:   foodID,
		Date:     date,
		Meal:     r.FormValue("meal"),
		Servings: servings,
		Notes:    r.FormValue("notes"),
	}

	if err := models.CreateEntry(entry); err != nil {
		httpError(w, err, http.StatusInternalServerError)
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

	dateRaw, err := models.DeleteEntry(id, user.ID)
	if err != nil {
		http.Error(w, "entry not found", http.StatusNotFound)
		return
	}

	date := normalizeDate(dateRaw)
	http.Redirect(w, r, "/?date="+date+"&deleted=entry&id="+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func RestoreEntry(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	dateRaw, err := models.RestoreEntry(id, user.ID)
	if err != nil {
		http.Error(w, "entry not found", http.StatusNotFound)
		return
	}

	date := normalizeDate(dateRaw)
	http.Redirect(w, r, "/?date="+date, http.StatusSeeOther)
}

func GetEntry(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := auth.GetUserFromContext(r.Context())

		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		entry, err := models.GetEntry(id, user.ID)
		if errors.Is(err, models.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}

		foods, err := models.GetAllFoods()
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Title": "Edit Entry",
			"Entry": entry,
			"Foods": foods,
			"Meals": []string{"breakfast", "lunch", "dinner", "snack"},
			"User":  user,
		}

		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			httpError(w, err, http.StatusInternalServerError)
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

	if err := r.ParseMultipartForm(1 << 20); err != nil {
		if parseErr := r.ParseForm(); parseErr != nil {
			httpError(w, parseErr, http.StatusBadRequest)
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

	if err := models.UpdateEntryServings(id, user.ID, servings); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			http.Error(w, "entry not found", http.StatusNotFound)
			return
		}
		httpError(w, err, http.StatusInternalServerError)
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
		httpError(w, parseErr, http.StatusBadRequest)
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

	entry := &models.Entry{
		ID:       id,
		UserID:   user.ID,
		FoodID:   foodID,
		Meal:     r.FormValue("meal"),
		Servings: servings,
		Notes:    r.FormValue("notes"),
	}

	if err := models.UpdateEntry(entry); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			http.Error(w, "entry not found", http.StatusNotFound)
			return
		}
		httpError(w, err, http.StatusInternalServerError)
		return
	}

	date, _ := models.GetEntryDate(id, user.ID)
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	http.Redirect(w, r, "/?date="+date, http.StatusSeeOther)
}

// normalizeDate parses various date formats and returns YYYY-MM-DD.
func normalizeDate(date string) string {
	if date == "" {
		return time.Now().Format("2006-01-02")
	}
	if t, err := time.Parse(time.RFC3339, date); err == nil {
		return t.Format("2006-01-02")
	}
	if t, err := time.Parse("2006-01-02T15:04:05Z", date); err == nil {
		return t.Format("2006-01-02")
	}
	if t, err := time.Parse("2006-01-02", date); err == nil {
		return t.Format("2006-01-02")
	}
	if len(date) >= 10 {
		return date[:10]
	}
	return date
}
