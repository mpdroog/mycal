// Dashboard page TypeScript

interface SearchItem {
    type: "food" | "ingredient";
    id: number;
    name: string;
    calories: number;
    serving_type?: "weight" | "unit";
    serving_size?: string;
}

interface Goals {
    calories: number;
    protein: number;
    carbs: number;
    fat: number;
}

interface PrevTotals {
    cal: number | null;
    pro: number | null;
    carb: number | null;
    fat: number | null;
}

(function(): void {
    "use strict";

    // Utility: Debounce function to limit how often a function is called
    function debounce<T extends (...args: unknown[]) => void>(fn: T, delay: number): (...args: Parameters<T>) => void {
        let timeoutId: ReturnType<typeof setTimeout> | undefined;
        return function(this: unknown, ...args: Parameters<T>): void {
            clearTimeout(timeoutId);
            timeoutId = setTimeout(() => { fn.apply(this, args); }, delay);
        };
    }

    // Initialize progress bar widths from data attributes (CSP-safe)
    document.querySelectorAll<HTMLElement>(".progress-bar[data-width]").forEach((bar) => {
        const width = bar.dataset["width"];
        if (width) {
            bar.style.width = `${width}%`;
        }
    });

    // Read goals from config
    const configEl = document.getElementById("dashboardConfig");
    let goals: Goals = { calories: 2000, protein: 150, carbs: 250, fat: 65 };

    if (configEl?.textContent) {
        const config = JSON.parse(configEl.textContent) as { goals?: Goals };
        if (config.goals) {
            goals = config.goals;
        }
    }

    // Fuzzy search via API
    async function searchItems(query: string): Promise<SearchItem[]> {
        if (!query.trim()) return [];
        const res = await fetch(`/search?q=${encodeURIComponent(query)}`);
        if (!res.ok) return [];
        return res.json() as Promise<SearchItem[]>;
    }

    const itemSearch = document.getElementById("itemSearch") as HTMLInputElement | null;
    const searchResults = document.getElementById("searchResults");
    const foodIdInput = document.getElementById("foodIdInput") as HTMLInputElement | null;
    const ingredientIdInput = document.getElementById("ingredientIdInput") as HTMLInputElement | null;

    function selectItem(item: SearchItem): void {
        if (!itemSearch || !searchResults || !foodIdInput || !ingredientIdInput) return;

        itemSearch.value = item.name;
        searchResults.classList.add("d-none");

        if (item.type === "food") {
            foodIdInput.value = String(item.id);
            ingredientIdInput.value = "";
        } else {
            ingredientIdInput.value = String(item.id);
            foodIdInput.value = "";
        }

        // Update servings input based on serving type
        const servingsInput = document.getElementById("servingsInput") as HTMLInputElement | null;
        const servingsLabel = document.getElementById("servingsLabel");

        if (servingsInput && servingsLabel) {
            const servingType = item.serving_type || "weight";
            const servingSize = item.serving_size || "100g";

            if (servingType === "unit") {
                servingsInput.value = "1";
                servingsInput.step = "1";
                servingsInput.min = "1";
                servingsLabel.textContent = "× " + servingSize;
            } else {
                servingsInput.value = "100";
                servingsInput.step = "10";
                servingsInput.min = "10";
                servingsLabel.textContent = "g";
            }
        }
    }

    function escapeHtml(text: string): string {
        const div = document.createElement("div");
        div.textContent = text;
        return div.innerHTML;
    }

    if (itemSearch && searchResults) {
        // Debounced search handler using API
        const handleSearch = debounce(async function(): Promise<void> {
            const query = itemSearch.value.trim();

            if (query.length === 0) {
                searchResults.classList.add("d-none");
                searchResults.innerHTML = "";
                return;
            }

            const results = await searchItems(query);

            if (results.length === 0) {
                searchResults.innerHTML = '<div class="p-3 text-secondary">No matches found</div>';
                searchResults.classList.remove("d-none");
                return;
            }

            searchResults.innerHTML = results.map((item, index) => {
                const icon = item.type === "food" ? "\u{1F37D}\u{FE0F}" : "\u{1F957}";
                const typeLabel = item.type === "food" ? "Food" : "Ingredient";
                const servingInfo = item.serving_size ? `/${item.serving_size}` : "";
                return `<div class="search-result p-2 border-bottom" data-index="${String(index)}">` +
                    '<div class="d-flex align-items-center gap-2">' +
                    `<span>${icon}</span>` +
                    '<div class="flex-grow-1">' +
                    `<div class="fw-medium">${escapeHtml(item.name)}</div>` +
                    `<small class="text-secondary">${typeLabel} \u00B7 ${String(item.calories)} kcal${servingInfo}</small>` +
                    '</div></div></div>';
            }).join("");

            searchResults.classList.remove("d-none");
            searchResults.dataset["results"] = JSON.stringify(results);
        }, 200);
        itemSearch.addEventListener("input", handleSearch);

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

            const results = JSON.parse(resultsJson) as SearchItem[];
            const item = results[index];
            if (item) {
                selectItem(item);
            }
        });

        // Handle keyboard navigation
        itemSearch.addEventListener("keydown", function(e: KeyboardEvent): void {
            const items = searchResults.querySelectorAll<HTMLElement>(".search-result");
            const activeItem = searchResults.querySelector<HTMLElement>(".search-result.active");
            let activeIndex = -1;
            if (activeItem) {
                const idx = activeItem.dataset["index"];
                if (idx) {
                    activeIndex = parseInt(idx, 10);
                }
            }

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
                const results = JSON.parse(resultsJson) as SearchItem[];
                const item = results[activeIndex];
                if (item) {
                    selectItem(item);
                }
            } else if (e.key === "Escape") {
                searchResults.classList.add("d-none");
            }
        });

        // Close results when clicking outside
        document.addEventListener("click", function(e: Event): void {
            const target = e.target as Node;
            if (!itemSearch.contains(target) && !searchResults.contains(target)) {
                searchResults.classList.add("d-none");
            }
        });
    }

    // Validate form before submit
    const addEntryForm = document.getElementById("addEntryForm");
    if (addEntryForm && foodIdInput && ingredientIdInput && itemSearch) {
        addEntryForm.addEventListener("submit", function(e: Event): void {
            if (!foodIdInput.value && !ingredientIdInput.value) {
                e.preventDefault();
                itemSearch.focus();
                showError("Please select a food or ingredient", "Use the search box to find and select an item before adding.");
            }
        });
    }

    // Date picker navigation
    const datePicker = document.getElementById("datePicker") as HTMLInputElement | null;
    if (datePicker) {
        datePicker.addEventListener("change", function(): void {
            location.href = "/?date=" + this.value;
        });
    }

    // Toggle and servings functionality (requires entries)
    const entriesList = document.getElementById("entriesList");
    if (!entriesList) return;

    // Store previous values for animation
    let prevTotals: PrevTotals = { cal: null, pro: null, carb: null, fat: null };

    function animatePop(element: Element): void {
        element.classList.remove("value-pop");
        void (element as HTMLElement).offsetWidth; // Trigger reflow
        element.classList.add("value-pop");
    }

    function updateTotals(): void {
        let totalCal = 0, totalPro = 0, totalCarb = 0, totalFat = 0;

        document.querySelectorAll<HTMLElement>(".entry-row").forEach((row) => {
            const checkbox = row.querySelector<HTMLInputElement>(".entry-toggle");
            const servingsInput = row.querySelector<HTMLInputElement>(".servings-input");
            const caloriesEl = row.querySelector(".entry-calories");
            const saveBtn = row.querySelector(".save-btn");

            if (!checkbox || !servingsInput || !caloriesEl || !saveBtn) return;

            const inputValue = parseFloat(servingsInput.value) || 0;
            const servingType = row.dataset["servingType"] || "weight";

            // Convert input value to servings based on serving type
            // Weight-based: input is grams, servings = grams / 100
            // Unit-based: input is count, servings = count
            const servings = servingType === "unit" ? inputValue : inputValue / 100;

            // Calculate this row's nutrition
            const baseCal = parseInt(row.dataset["baseCalories"] ?? "0", 10);
            const basePro = parseFloat(row.dataset["baseProtein"] ?? "0");
            const baseCarb = parseFloat(row.dataset["baseCarbs"] ?? "0");
            const baseFat = parseFloat(row.dataset["baseFat"] ?? "0");

            const rowCal = Math.round(baseCal * servings);
            const rowPro = basePro * servings;
            const rowCarb = baseCarb * servings;
            const rowFat = baseFat * servings;

            // Update row's calorie display
            caloriesEl.textContent = `${String(rowCal)} kcal`;

            if (checkbox.checked) {
                totalCal += rowCal;
                totalPro += rowPro;
                totalCarb += rowCarb;
                totalFat += rowFat;
            }

            // Visual feedback for disabled items
            row.classList.toggle("disabled", !checkbox.checked);

            // Show/hide save button if input value changed
            // originalServings is stored as servings (not input value), so convert for comparison
            const originalServings = parseFloat(row.dataset["originalServings"] ?? "0");
            const originalInputValue = servingType === "unit" ? originalServings : originalServings * 100;
            if (Math.abs(inputValue - originalInputValue) > 0.001) {
                saveBtn.classList.remove("d-none");
            } else {
                saveBtn.classList.add("d-none");
            }
        });

        // Check if values changed for animations
        const calChanged = prevTotals.cal !== null && prevTotals.cal !== totalCal;
        const proChanged = prevTotals.pro !== null && Math.round(prevTotals.pro) !== Math.round(totalPro);
        const carbChanged = prevTotals.carb !== null && Math.round(prevTotals.carb) !== Math.round(totalCarb);
        const fatChanged = prevTotals.fat !== null && Math.round(prevTotals.fat) !== Math.round(totalFat);

        // Update header displays (eaten calories only - index 0)
        // Index 1 is remaining (updated separately), index 2 is goal (static)
        const headerValues = document.querySelectorAll(".summary-header .fs-4.fw-bold");
        if (headerValues[0]) {
            headerValues[0].textContent = String(totalCal);
            if (calChanged) animatePop(headerValues[0]);
        }

        // Update macro displays with animations
        const macroValues = document.querySelectorAll(".macro-progress .fw-bold");
        if (macroValues.length >= 3) {
            const carbEl = macroValues[0];
            const proEl = macroValues[1];
            const fatEl = macroValues[2];

            if (carbEl) {
                carbEl.textContent = String(Math.round(totalCarb));
                if (carbChanged) animatePop(carbEl);
            }
            if (proEl) {
                proEl.textContent = String(Math.round(totalPro));
                if (proChanged) animatePop(proEl);
            }
            if (fatEl) {
                fatEl.textContent = String(Math.round(totalFat));
                if (fatChanged) animatePop(fatEl);
            }
        }

        // Update progress bars (CSS transition handles animation)
        const progressBars = document.querySelectorAll<HTMLElement>(".macro-progress .progress-bar");
        if (progressBars.length >= 3) {
            const carbBar = progressBars[0];
            const proBar = progressBars[1];
            const fatBar = progressBars[2];

            if (carbBar) carbBar.style.width = `${String(Math.min(100, (totalCarb / goals.carbs) * 100))}%`;
            if (proBar) proBar.style.width = `${String(Math.min(100, (totalPro / goals.protein) * 100))}%`;
            if (fatBar) fatBar.style.width = `${String(Math.min(100, (totalFat / goals.fat) * 100))}%`;
        }

        // Update calorie ring with animation
        const ringProgress = document.querySelector(".calorie-ring-progress");
        const ringSvg = document.querySelector(".calorie-ring-svg");
        if (ringProgress) {
            const pct = Math.min(100, (totalCal / goals.calories) * 100);
            ringProgress.setAttribute("stroke-dasharray", `${String(pct)}, 100`);

            if (calChanged && ringSvg) {
                ringSvg.classList.remove("ring-pulse");
                void (ringSvg as HTMLElement).offsetWidth;
                ringSvg.classList.add("ring-pulse");
            }
        }

        // Update remaining calories display in ring
        const remainingCal = document.getElementById("remainingCal");
        if (remainingCal) {
            const remaining = goals.calories - totalCal;
            remainingCal.textContent = String(remaining);
            remainingCal.classList.toggle("text-danger", remaining < 0);
            if (calChanged) animatePop(remainingCal);
        }

        // Update floating summary with animations
        const floatCal = document.getElementById("floatCal");
        const floatPro = document.getElementById("floatPro");
        const floatCarb = document.getElementById("floatCarb");
        const floatFat = document.getElementById("floatFat");

        if (floatCal) {
            floatCal.textContent = String(totalCal);
            if (calChanged) animatePop(floatCal);
        }
        if (floatPro) {
            floatPro.textContent = String(Math.round(totalPro));
            if (proChanged) animatePop(floatPro);
        }
        if (floatCarb) {
            floatCarb.textContent = String(Math.round(totalCarb));
            if (carbChanged) animatePop(floatCarb);
        }
        if (floatFat) {
            floatFat.textContent = String(Math.round(totalFat));
            if (fatChanged) animatePop(floatFat);
        }

        // Store current values for next comparison
        prevTotals = { cal: totalCal, pro: totalPro, carb: totalCarb, fat: totalFat };
    }

    entriesList.addEventListener("change", function(e: Event): void {
        const target = e.target as HTMLElement;
        if (target.classList.contains("entry-toggle")) {
            updateTotals();
        }
    });

    entriesList.addEventListener("input", function(e: Event): void {
        const target = e.target as HTMLElement;
        if (target.classList.contains("servings-input")) {
            updateTotals();
        }
    });

    // Save button click - update entry servings
    entriesList.addEventListener("click", function(e: Event): void {
        const target = e.target as HTMLElement;
        if (target.classList.contains("save-btn")) {
            const row = target.closest<HTMLElement>(".entry-row");
            if (!row) return;

            const entryId = row.dataset["id"];
            const servingsInput = row.querySelector<HTMLInputElement>(".servings-input");
            if (!entryId || !servingsInput) return;

            const inputValue = parseFloat(servingsInput.value) || 0;
            const servingType = row.dataset["servingType"] || "weight";

            // Convert input value to servings based on serving type
            // Weight-based: input is grams, servings = grams / 100
            // Unit-based: input is count, servings = count
            const servings = servingType === "unit" ? inputValue : inputValue / 100;

            // Send update request
            const formData = new FormData();
            formData.append("servings", String(servings));

            fetch(`/entries/${entryId}/servings`, {
                method: "POST",
                body: formData
            }).then((response) => {
                if (response.ok) {
                    row.dataset["originalServings"] = String(servings);
                    target.classList.add("d-none");
                }
            }).catch(() => {
                // Error handled by global fetch wrapper
            });
        }
    });
})();
