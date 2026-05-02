package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/mpdroog/mycal/auth"
	"github.com/mpdroog/mycal/db"
	"github.com/mpdroog/mycal/models"
)

// GetProfileForUser returns the user's profile settings.
func GetProfileForUser(userID int64) (*models.Profile, error) {
	var p models.Profile

	err := db.DB.QueryRow(`
		SELECT calories_goal, protein_goal, carbs_goal, fat_goal
		FROM profile WHERE user_id = ?
	`, userID).Scan(&p.CaloriesGoal, &p.ProteinGoal, &p.CarbsGoal, &p.FatGoal)
	if err != nil {
		// Return default profile if not found
		return &models.Profile{
			CaloriesGoal: 2000,
			ProteinGoal:  150,
			CarbsGoal:    250,
			FatGoal:      65,
		}, nil
	}

	return &p, nil
}

func Profile(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := auth.GetUserFromContext(r.Context())

		if r.Method == http.MethodGet {
			profile, err := GetProfileForUser(user.ID)
			if err != nil {
				httpError(w, err, http.StatusInternalServerError)

				return
			}

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

		// POST - update profile
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

		// Upsert profile for user
		_, err = db.DB.Exec(`
			INSERT INTO profile (user_id, calories_goal, protein_goal, carbs_goal, fat_goal)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(user_id) DO UPDATE SET
				calories_goal = excluded.calories_goal,
				protein_goal = excluded.protein_goal,
				carbs_goal = excluded.carbs_goal,
				fat_goal = excluded.fat_goal
		`, user.ID, calories, protein, carbs, fat)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)

			return
		}

		http.Redirect(w, r, "/profile", http.StatusSeeOther)
	}
}
