// Food form page TypeScript

interface IngredientItem {
    id: number;
    name: string;
    calories: number;
    protein: number;
    carbs: number;
    fat: number;
    serving: string;
}

interface IngredientFormData {
    ingredient_id: number;
    amount_grams: number;
}

interface RemovedIngredient {
    row: HTMLElement;
    nextSibling: Node | null;
    timeoutId: ReturnType<typeof setTimeout>;
}

(function(): void {
    "use strict";

    // Read configuration from script content
    const configEl = document.getElementById("foodFormConfig");
    if (!configEl || !configEl.textContent) return;

    interface FoodFormConfig {
        ingredients: Array<{
            id: number;
            name: string;
            calories: number;
            protein: number;
            carbs: number;
            fat: number;
            serving_size: string;
        }>;
    }

    const config = JSON.parse(configEl.textContent) as FoodFormConfig;

    // Map to expected format
    const ingredientsData: IngredientItem[] = (config.ingredients || []).map((i) => ({
        id: i.id,
        name: i.name,
        calories: i.calories,
        protein: i.protein,
        carbs: i.carbs,
        fat: i.fat,
        serving: i.serving_size
    }));

    // Initialize Fuse.js
    const ingredientFuse = new Fuse<IngredientItem>(ingredientsData, {
        keys: ["name"],
        threshold: 0.4,
        distance: 100,
        includeScore: true,
        minMatchCharLength: 1,
        ignoreLocation: true
    });

    const ingredientsList = document.getElementById("ingredientsList");
    const ingredientSearch = document.getElementById("ingredientSearch") as HTMLInputElement | null;
    const searchResults = document.getElementById("ingredientSearchResults");
    const form = document.getElementById("foodForm") as HTMLFormElement | null;
    const ingredientsJson = document.getElementById("ingredientsJson") as HTMLInputElement | null;
    const nameInput = document.getElementById("name") as HTMLInputElement | null;

    if (!ingredientsList || !searchResults || !ingredientsJson) return;

    // Track unsaved changes
    let hasUnsavedChanges = false;
    let removedIngredient: RemovedIngredient | null = null;

    // Store initial state for comparison
    const initialName = nameInput?.value ?? "";
    const initialIngredients = new Set<string>();
    ingredientsList.querySelectorAll<HTMLElement>(".ingredient-row").forEach((row) => {
        const id = row.dataset["id"];
        const amountInput = row.querySelector<HTMLInputElement>(".amount-input");
        if (id && amountInput) {
            initialIngredients.add(`${id}:${amountInput.value}`);
        }
    });

    function checkForChanges(): void {
        if (!nameInput) return;

        // Check name change
        if (nameInput.value !== initialName) {
            setUnsavedChanges(true);
            return;
        }

        // Check ingredients
        const currentIngredients = new Set<string>();
        ingredientsList!.querySelectorAll<HTMLElement>(".ingredient-row").forEach((row) => {
            const id = row.dataset["id"];
            const amountInput = row.querySelector<HTMLInputElement>(".amount-input");
            if (id && amountInput) {
                currentIngredients.add(`${id}:${amountInput.value}`);
            }
        });

        // Compare sets
        if (currentIngredients.size !== initialIngredients.size) {
            setUnsavedChanges(true);
            return;
        }

        for (const item of currentIngredients) {
            if (!initialIngredients.has(item)) {
                setUnsavedChanges(true);
                return;
            }
        }

        setUnsavedChanges(false);
    }

    function setUnsavedChanges(value: boolean): void {
        hasUnsavedChanges = value;
        const indicator = document.getElementById("unsavedIndicator");
        if (indicator) {
            indicator.classList.toggle("d-none", !value);
        }
    }

    // Warn on navigation
    window.addEventListener("beforeunload", function(e: BeforeUnloadEvent): string | undefined {
        if (hasUnsavedChanges) {
            e.preventDefault();
            return "You have unsaved changes. Are you sure you want to leave?";
        }
        return undefined;
    });

    function escapeHtml(text: string): string {
        const div = document.createElement("div");
        div.textContent = text;
        return div.innerHTML;
    }

    function updateTotals(): void {
        const rows = ingredientsList!.querySelectorAll<HTMLElement>(".ingredient-row");
        let totalCal = 0, totalPro = 0, totalCarb = 0, totalFat = 0;

        rows.forEach((row) => {
            const amountInput = row.querySelector<HTMLInputElement>(".amount-input");
            if (!amountInput) return;

            const amount = parseFloat(amountInput.value) || 0;
            const ratio = amount / 100;
            totalCal += parseFloat(row.dataset["calories"] ?? "0") * ratio;
            totalPro += parseFloat(row.dataset["protein"] ?? "0") * ratio;
            totalCarb += parseFloat(row.dataset["carbs"] ?? "0") * ratio;
            totalFat += parseFloat(row.dataset["fat"] ?? "0") * ratio;
        });

        const totalCaloriesEl = document.getElementById("totalCalories");
        const totalProteinEl = document.getElementById("totalProtein");
        const totalCarbsEl = document.getElementById("totalCarbs");
        const totalFatEl = document.getElementById("totalFat");

        if (totalCaloriesEl) totalCaloriesEl.textContent = String(Math.round(totalCal));
        if (totalProteinEl) totalProteinEl.textContent = `${String(Math.round(totalPro))}g`;
        if (totalCarbsEl) totalCarbsEl.textContent = `${String(Math.round(totalCarb))}g`;
        if (totalFatEl) totalFatEl.textContent = `${String(Math.round(totalFat))}g`;
    }

    function collectIngredients(): void {
        const rows = ingredientsList!.querySelectorAll<HTMLElement>(".ingredient-row");
        const ingredients: IngredientFormData[] = [];

        rows.forEach((row) => {
            const idStr = row.dataset["id"];
            const amountInput = row.querySelector<HTMLInputElement>(".amount-input");
            if (!idStr || !amountInput) return;

            const id = parseInt(idStr, 10);
            const amount = parseFloat(amountInput.value) || 100;
            ingredients.push({ ingredient_id: id, amount_grams: amount });
        });

        ingredientsJson!.value = JSON.stringify(ingredients);
    }

    function addIngredient(item: IngredientItem): void {
        // Check if already added
        if (ingredientsList!.querySelector(`[data-id="${String(item.id)}"]`)) {
            showError("Ingredient already added", "This ingredient is already in the list.");
            return;
        }

        const row = document.createElement("div");
        row.className = "d-flex align-items-center gap-2 mb-2 ingredient-row";
        row.dataset["id"] = String(item.id);
        row.dataset["calories"] = String(item.calories);
        row.dataset["protein"] = String(item.protein);
        row.dataset["carbs"] = String(item.carbs);
        row.dataset["fat"] = String(item.fat);
        row.innerHTML = '<div class="flex-grow-1">' +
            `<span class="fw-medium">${escapeHtml(item.name)}</span>` +
            `<small class="text-secondary">(${String(item.calories)} kcal/${escapeHtml(item.serving)})</small>` +
            '</div>' +
            '<input type="number" class="form-control form-control-sm amount-input amount-input-width" value="100" step="1" min="1" placeholder="g">' +
            '<span class="text-secondary">g</span>' +
            '<button type="button" class="btn btn-outline-danger btn-sm remove-ingredient">\u00D7</button>';
        ingredientsList!.appendChild(row);

        if (ingredientSearch) {
            ingredientSearch.value = "";
        }
        searchResults!.classList.add("d-none");
        updateTotals();
        checkForChanges();
    }

    // Search input handler
    if (ingredientSearch) {
        ingredientSearch.addEventListener("input", function(this: HTMLInputElement): void {
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

            searchResults.innerHTML = results.map((result, index) => {
                const item = result.item;
                return `<div class="search-result p-2 border-bottom" data-index="${String(index)}">` +
                    `<div class="fw-medium">${escapeHtml(item.name)}</div>` +
                    `<small class="text-secondary">${String(item.calories)} kcal/${escapeHtml(item.serving)} \u00B7 P: ${String(item.protein)}g C: ${String(item.carbs)}g F: ${String(item.fat)}g</small>` +
                    '</div>';
            }).join("");

            searchResults.classList.remove("d-none");
            searchResults.dataset["results"] = JSON.stringify(results.map((r) => r.item));
        });

        // Handle result click
        searchResults.addEventListener("click", function(this: HTMLElement, e: Event): void {
            const target = e.target as HTMLElement;
            const resultEl = target.closest<HTMLElement>(".search-result");
            if (!resultEl) return;

            const indexStr = resultEl.dataset["index"];
            if (!indexStr) return;

            const index = parseInt(indexStr, 10);
            const resultsJson = this.dataset["results"];
            if (!resultsJson) return;

            const results = JSON.parse(resultsJson) as IngredientItem[];
            const item = results[index];
            if (item) {
                addIngredient(item);
            }
        });

        // Keyboard navigation
        ingredientSearch.addEventListener("keydown", function(e: KeyboardEvent): void {
            const items = searchResults.querySelectorAll<HTMLElement>(".search-result");
            let activeIndex = -1;

            items.forEach((item, i) => {
                if (item.classList.contains("active")) activeIndex = i;
            });

            if (e.key === "ArrowDown") {
                e.preventDefault();
                activeIndex = Math.min(activeIndex + 1, items.length - 1);
                items.forEach((item, i) => {
                    item.classList.toggle("active", i === activeIndex);
                });
            } else if (e.key === "ArrowUp") {
                e.preventDefault();
                activeIndex = Math.max(activeIndex - 1, 0);
                items.forEach((item, i) => {
                    item.classList.toggle("active", i === activeIndex);
                });
            } else if (e.key === "Enter" && activeIndex >= 0) {
                e.preventDefault();
                const resultsJson = searchResults.dataset["results"] ?? "[]";
                const results = JSON.parse(resultsJson) as IngredientItem[];
                const item = results[activeIndex];
                if (item) {
                    addIngredient(item);
                }
            } else if (e.key === "Escape") {
                searchResults.classList.add("d-none");
            }
        });

        // Close results when clicking outside
        document.addEventListener("click", function(e: Event): void {
            const target = e.target as Node;
            if (!ingredientSearch.contains(target) && !searchResults.contains(target)) {
                searchResults.classList.add("d-none");
            }
        });
    }

    function getOrCreateUndoContainer(): HTMLElement {
        let container = document.getElementById("undoContainer");
        if (!container) {
            container = document.createElement("div");
            container.id = "undoContainer";
            container.className = "mb-2";
            ingredientsList!.parentNode!.insertBefore(container, ingredientsList);
        }
        return container;
    }

    function hideUndoNotification(): void {
        const container = document.getElementById("undoContainer");
        if (container) {
            container.innerHTML = "";
        }
    }

    function showUndoNotification(ingredientName: string): void {
        const container = getOrCreateUndoContainer();
        container.innerHTML =
            '<div class="alert alert-warning py-2 px-3 d-flex align-items-center justify-content-between mb-0">' +
            `<span>Removed <strong>${escapeHtml(ingredientName)}</strong></span>` +
            '<button type="button" class="btn btn-sm btn-warning undo-btn">Undo</button>' +
            '</div>';
    }

    function removeIngredient(row: HTMLElement): void {
        // If there's already a pending removal, finalize it
        if (removedIngredient) {
            clearTimeout(removedIngredient.timeoutId);
            removedIngredient = null;
            hideUndoNotification();
        }

        const nameEl = row.querySelector<HTMLElement>(".fw-medium");
        const ingredientName = nameEl?.textContent ?? "Ingredient";
        const nextSibling = row.nextSibling;

        // Remove from DOM but keep in memory
        row.remove();
        updateTotals();
        checkForChanges();

        // Show undo notification
        showUndoNotification(ingredientName);

        // Set timeout to clear undo option
        const timeoutId = setTimeout(() => {
            removedIngredient = null;
            hideUndoNotification();
        }, 5000);

        removedIngredient = { row, nextSibling, timeoutId };
    }

    function undoRemoval(): void {
        if (!removedIngredient) return;

        clearTimeout(removedIngredient.timeoutId);

        // Re-insert the row at original position
        if (removedIngredient.nextSibling) {
            ingredientsList!.insertBefore(removedIngredient.row, removedIngredient.nextSibling);
        } else {
            ingredientsList!.appendChild(removedIngredient.row);
        }

        removedIngredient = null;
        hideUndoNotification();
        updateTotals();
        checkForChanges();
    }

    ingredientsList.addEventListener("click", function(e: Event): void {
        const target = e.target as HTMLElement;
        if (target.classList.contains("remove-ingredient")) {
            const row = target.closest<HTMLElement>(".ingredient-row");
            if (row) {
                removeIngredient(row);
            }
        }
    });

    // Handle undo button click (delegated from parent since container is dynamic)
    document.addEventListener("click", function(e: Event): void {
        const target = e.target as HTMLElement;
        if (target.classList.contains("undo-btn") && target.closest("#undoContainer")) {
            undoRemoval();
        }
    });

    // Update totals when amount changes
    ingredientsList.addEventListener("input", function(e: Event): void {
        const target = e.target as HTMLElement;
        if (target.classList.contains("amount-input")) {
            updateTotals();
            checkForChanges();
        }
    });

    // Track name changes
    if (nameInput) {
        nameInput.addEventListener("input", function(): void {
            checkForChanges();
        });
    }

    if (form) {
        form.addEventListener("submit", function(): void {
            // Finalize any pending removal
            if (removedIngredient) {
                clearTimeout(removedIngredient.timeoutId);
                removedIngredient = null;
            }
            collectIngredients();
            // Clear flag so beforeunload doesn't trigger
            hasUnsavedChanges = false;
        });
    }

    updateTotals();
})();
