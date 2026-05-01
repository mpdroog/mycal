// Food form page JavaScript
(function() {
    "use strict";

    // Read configuration from data attributes
    const configEl = document.getElementById("foodFormConfig");
    if (!configEl) return;

    const ingredientsData = JSON.parse(configEl.dataset.ingredients || "[]");

    // Initialize Fuse.js
    const ingredientFuse = new Fuse(ingredientsData, {
        keys: ["name"],
        threshold: 0.4,
        distance: 100,
        includeScore: true,
        minMatchCharLength: 1,
        ignoreLocation: true
    });

    const ingredientsList = document.getElementById("ingredientsList");
    const ingredientSearch = document.getElementById("ingredientSearch");
    const searchResults = document.getElementById("ingredientSearchResults");
    const form = document.getElementById("foodForm");
    const ingredientsJson = document.getElementById("ingredientsJson");

    function updateTotals() {
        const rows = ingredientsList.querySelectorAll(".ingredient-row");
        let totalCal = 0, totalPro = 0, totalCarb = 0, totalFat = 0;

        rows.forEach(function(row) {
            const amount = parseFloat(row.querySelector(".amount-input").value) || 0;
            const ratio = amount / 100;
            totalCal += (parseFloat(row.dataset.calories) || 0) * ratio;
            totalPro += (parseFloat(row.dataset.protein) || 0) * ratio;
            totalCarb += (parseFloat(row.dataset.carbs) || 0) * ratio;
            totalFat += (parseFloat(row.dataset.fat) || 0) * ratio;
        });

        document.getElementById("totalCalories").textContent = Math.round(totalCal);
        document.getElementById("totalProtein").textContent = Math.round(totalPro) + "g";
        document.getElementById("totalCarbs").textContent = Math.round(totalCarb) + "g";
        document.getElementById("totalFat").textContent = Math.round(totalFat) + "g";
    }

    function collectIngredients() {
        const rows = ingredientsList.querySelectorAll(".ingredient-row");
        const ingredients = [];
        rows.forEach(function(row) {
            const id = parseInt(row.dataset.id, 10);
            const amount = parseFloat(row.querySelector(".amount-input").value) || 100;
            ingredients.push({ ingredient_id: id, amount_grams: amount });
        });
        ingredientsJson.value = JSON.stringify(ingredients);
    }

    function addIngredient(item) {
        // Check if already added
        if (ingredientsList.querySelector('[data-id="' + item.id + '"]')) {
            showError("Ingredient already added", "This ingredient is already in the list.");
            return;
        }

        const row = document.createElement("div");
        row.className = "d-flex align-items-center gap-2 mb-2 ingredient-row";
        row.dataset.id = item.id;
        row.dataset.calories = item.calories;
        row.dataset.protein = item.protein;
        row.dataset.carbs = item.carbs;
        row.dataset.fat = item.fat;
        row.innerHTML = '<div class="flex-grow-1">' +
            '<span class="fw-medium">' + item.name + '</span>' +
            '<small class="text-secondary">(' + item.calories + ' kcal/' + item.serving + ')</small>' +
            '</div>' +
            '<input type="number" class="form-control form-control-sm amount-input amount-input-width" value="100" step="1" min="1" placeholder="g">' +
            '<span class="text-secondary">g</span>' +
            '<button type="button" class="btn btn-outline-danger btn-sm remove-ingredient">\u00D7</button>';
        ingredientsList.appendChild(row);
        ingredientSearch.value = "";
        searchResults.classList.add("d-none");
        updateTotals();
    }

    // Search input handler
    if (ingredientSearch) {
        ingredientSearch.addEventListener("input", function() {
            const query = this.value.trim();

            if (query.length === 0) {
                searchResults.classList.add("d-none");
                searchResults.innerHTML = "";
                return;
            }

            const results = ingredientFuse.search(query).slice(0, 8);

            if (results.length === 0) {
                searchResults.innerHTML = '<div class="p-3 text-secondary">No matches found</div>';
                searchResults.classList.remove("d-none");
                return;
            }

            searchResults.innerHTML = results.map(function(result, index) {
                const item = result.item;
                return '<div class="search-result p-2 border-bottom" data-index="' + index + '">' +
                    '<div class="fw-medium">' + item.name + '</div>' +
                    '<small class="text-secondary">' + item.calories + ' kcal/' + item.serving + ' \u00B7 P: ' + item.protein + 'g C: ' + item.carbs + 'g F: ' + item.fat + 'g</small>' +
                    '</div>';
            }).join("");

            searchResults.classList.remove("d-none");
            searchResults.dataset.results = JSON.stringify(results.map(function(r) { return r.item; }));
        });

        // Handle result click
        searchResults.addEventListener("click", function(e) {
            const resultEl = e.target.closest(".search-result");
            if (!resultEl) return;

            const index = parseInt(resultEl.dataset.index, 10);
            const results = JSON.parse(this.dataset.results);
            addIngredient(results[index]);
        });

        // Keyboard navigation
        ingredientSearch.addEventListener("keydown", function(e) {
            const items = searchResults.querySelectorAll(".search-result");
            let activeIndex = -1;
            items.forEach(function(item, i) {
                if (item.classList.contains("active")) activeIndex = i;
            });

            if (e.key === "ArrowDown") {
                e.preventDefault();
                activeIndex = Math.min(activeIndex + 1, items.length - 1);
                items.forEach(function(item, i) {
                    item.classList.toggle("active", i === activeIndex);
                    item.classList.toggle("active", i === activeIndex);
                });
            } else if (e.key === "ArrowUp") {
                e.preventDefault();
                activeIndex = Math.max(activeIndex - 1, 0);
                items.forEach(function(item, i) {
                    item.classList.toggle("active", i === activeIndex);
                    item.classList.toggle("active", i === activeIndex);
                });
            } else if (e.key === "Enter" && activeIndex >= 0) {
                e.preventDefault();
                const results = JSON.parse(searchResults.dataset.results || "[]");
                if (results[activeIndex]) {
                    addIngredient(results[activeIndex]);
                }
            } else if (e.key === "Escape") {
                searchResults.classList.add("d-none");
            }
        });

        // Close results when clicking outside
        document.addEventListener("click", function(e) {
            if (!ingredientSearch.contains(e.target) && !searchResults.contains(e.target)) {
                searchResults.classList.add("d-none");
            }
        });
    }

    if (ingredientsList) {
        ingredientsList.addEventListener("click", function(e) {
            if (e.target.classList.contains("remove-ingredient")) {
                e.target.closest(".ingredient-row").remove();
                updateTotals();
            }
        });

        // Update totals when amount changes
        ingredientsList.addEventListener("input", function(e) {
            if (e.target.classList.contains("amount-input")) {
                updateTotals();
            }
        });
    }

    if (form) {
        form.addEventListener("submit", function() {
            collectIngredients();
        });
    }

    updateTotals();
})();
