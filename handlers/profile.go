package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/mpdroog/mycal/db"
	"github.com/mpdroog/mycal/models"
)

// GetProfile returns the user's profile settings.
func GetProfile() (*models.Profile, error) {
	var p models.Profile

	err := db.DB.QueryRow(`
		SELECT calories_goal, protein_goal, carbs_goal, fat_goal
		FROM profile WHERE id = 1
	`).Scan(&p.CaloriesGoal, &p.ProteinGoal, &p.CarbsGoal, &p.FatGoal)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func Profile(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			profile, err := GetProfile()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}

			data := map[string]interface{}{
				"Title":   "Profile",
				"Profile": profile,
			}

			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

			return
		}

		// POST - update profile
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)

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

		_, err = db.DB.Exec(`
			UPDATE profile SET calories_goal = ?, protein_goal = ?, carbs_goal = ?, fat_goal = ?
			WHERE id = 1
		`, calories, protein, carbs, fat)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		http.Redirect(w, r, "/profile", http.StatusSeeOther)
	}
}
