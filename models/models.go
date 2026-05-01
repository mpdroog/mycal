package models

import (
	"database/sql"
	"time"
)

// ErrNotFound is returned when a database record is not found.
var ErrNotFound = sql.ErrNoRows

type Food struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Calories    int       `json:"calories"`
	Protein     float64   `json:"protein"`
	Carbs       float64   `json:"carbs"`
	Fat         float64   `json:"fat"`
	ServingSize string    `json:"serving_size"`
	CreatedAt   time.Time `json:"created_at"`
}

type Entry struct {
	ID        int64     `json:"id"`
	FoodID    int64     `json:"food_id"`
	Date      string    `json:"date"` // YYYY-MM-DD
	Meal      string    `json:"meal"` // breakfast, lunch, dinner, snack
	Servings  float64   `json:"servings"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`

	// Joined fields
	Food *Food `json:"food,omitempty"`
}

type MealTemplate struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Foods string `json:"foods"` // JSON array
}

type PlannedMeal struct {
	ID         int64  `json:"id"`
	Date       string `json:"date"`
	Meal       string `json:"meal"`
	TemplateID int64  `json:"template_id"`

	// Joined fields
	Template *MealTemplate `json:"template,omitempty"`
}

type DaySummary struct {
	Date     string  `json:"date"`
	Calories int     `json:"calories"`
	Protein  float64 `json:"protein"`
	Carbs    float64 `json:"carbs"`
	Fat      float64 `json:"fat"`
	Entries  []Entry `json:"entries"`
}
