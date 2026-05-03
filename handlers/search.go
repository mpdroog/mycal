package handlers

import (
	"net/http"

	"github.com/mpdroog/mycal/models"
)

// Search returns combined fuzzy search results for foods and ingredients.
func Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")

	results, err := models.FuzzySearch(q, 10)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}

	if results == nil {
		results = []models.SearchItem{}
	}
	writeJSON(w, results)
}
