package tmpl

import (
	"encoding/json"
	"html/template"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Templates holds all parsed page templates.
type Templates struct {
	Dashboard      *template.Template
	Ingredients    *template.Template
	IngredientForm *template.Template
	Foods          *template.Template
	FoodForm       *template.Template
	EntryForm      *template.Template
	Profile        *template.Template
	Login          *template.Template
	Setup          *template.Template
	AdminUsers     *template.Template
	AdminUserForm  *template.Template
}

// FuncMap returns template functions with the given version string.
func FuncMap(version string) template.FuncMap {
	return template.FuncMap{
		"json": func(v interface{}) template.JS {
			b, err := json.Marshal(v)
			if err != nil {
				return template.JS("null")
			}
			return template.JS(b)
		},
		"prevDay": func(date string) string {
			t, err := time.Parse("2006-01-02", date)
			if err != nil {
				return date
			}
			return t.AddDate(0, 0, -1).Format("2006-01-02")
		},
		"nextDay": func(date string) string {
			t, err := time.Parse("2006-01-02", date)
			if err != nil {
				return date
			}
			return t.AddDate(0, 0, 1).Format("2006-01-02")
		},
		"relativeDate": func(date string) string {
			t, err := time.Parse("2006-01-02", date)
			if err != nil {
				return ""
			}

			now := time.Now()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			target := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, now.Location())

			days := int(target.Sub(today).Hours() / 24)

			switch days {
			case 0:
				return "Today"
			case 1:
				return "Tomorrow"
			case -1:
				return "Yesterday"
			}

			weekday := t.Weekday().String()

			if days >= 2 && days <= 6 {
				return weekday
			}

			if days >= -6 && days <= -2 {
				return "Last " + weekday
			}

			if days > 6 && days <= 13 {
				return "Next " + weekday
			}

			weeks := days / 7
			if days < 0 {
				weeks = (-days) / 7
				if weeks == 1 {
					return weekday + ", 1 week ago"
				}
				return weekday + ", " + strconv.Itoa(weeks) + " weeks ago"
			}

			if weeks == 1 {
				return weekday + ", in 1 week"
			}
			return weekday + ", in " + strconv.Itoa(weeks) + " weeks"
		},
		"title": cases.Title(language.English).String,
		"multiply": func(a int, b float64) int {
			return int(float64(a) * b)
		},
		"divide": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"percentage": func(value, goal float64) int {
			if goal == 0 {
				return 0
			}
			pct := (value / goal) * 100
			if pct > 100 {
				return 100
			}
			return int(pct)
		},
		"intToFloat": func(i int) float64 {
			return float64(i)
		},
		"multiplyFloat": func(a, b float64) float64 {
			return a * b
		},
		"subtract": func(a, b int) int {
			return a - b
		},
		"version": func() string {
			return version
		},
	}
}

// Load parses all templates from the given directory.
func Load(dir, version string) (*Templates, error) {
	fm := FuncMap(version)
	base := filepath.Join(dir, "base.html")

	parse := func(name string) (*template.Template, error) {
		return template.New("").Funcs(fm).ParseFiles(base, filepath.Join(dir, name))
	}

	dashboard, err := parse("dashboard.html")
	if err != nil {
		return nil, err
	}

	ingredients, err := parse("ingredients.html")
	if err != nil {
		return nil, err
	}

	ingredientForm, err := parse("ingredient_form.html")
	if err != nil {
		return nil, err
	}

	foods, err := parse("foods.html")
	if err != nil {
		return nil, err
	}

	foodForm, err := parse("food_form.html")
	if err != nil {
		return nil, err
	}

	entryForm, err := parse("entry_form.html")
	if err != nil {
		return nil, err
	}

	profile, err := parse("profile.html")
	if err != nil {
		return nil, err
	}

	login, err := parse("login.html")
	if err != nil {
		return nil, err
	}

	setup, err := parse("setup.html")
	if err != nil {
		return nil, err
	}

	adminUsers, err := parse("admin_users.html")
	if err != nil {
		return nil, err
	}

	adminUserForm, err := parse("admin_user_form.html")
	if err != nil {
		return nil, err
	}

	return &Templates{
		Dashboard:      dashboard,
		Ingredients:    ingredients,
		IngredientForm: ingredientForm,
		Foods:          foods,
		FoodForm:       foodForm,
		EntryForm:      entryForm,
		Profile:        profile,
		Login:          login,
		Setup:          setup,
		AdminUsers:     adminUsers,
		AdminUserForm:  adminUserForm,
	}, nil
}
