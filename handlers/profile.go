package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/mpdroog/mycal/auth"
	"github.com/mpdroog/mycal/models"
)

func Profile(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := auth.GetUserFromContext(r.Context())

		if r.Method == http.MethodGet {
			profile := models.GetProfile(user.ID)

			data := map[string]interface{}{
				"Title":   "Profile",
				"Profile": profile,
				"User":    user,
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

		calories, err := strconv.Atoi(r.FormValue("calories_goal"))
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid calories: %v", err), http.StatusBadRequest)
			return
		}

		protein, err := strconv.ParseFloat(r.FormValue("protein_goal"), 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid protein: %v", err), http.StatusBadRequest)
			return
		}

		carbs, err := strconv.ParseFloat(r.FormValue("carbs_goal"), 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid carbs: %v", err), http.StatusBadRequest)
			return
		}

		fat, err := strconv.ParseFloat(r.FormValue("fat_goal"), 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid fat: %v", err), http.StatusBadRequest)
			return
		}

		profile := &models.Profile{
			UserID:       user.ID,
			CaloriesGoal: calories,
			ProteinGoal:  protein,
			CarbsGoal:    carbs,
			FatGoal:      fat,
		}

		if err := models.SaveProfile(profile); err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/profile", http.StatusSeeOther)
	}
}
