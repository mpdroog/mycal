"use strict";
// Dashboard page TypeScript
(function () {
    "use strict";
    // Utility: Debounce function to limit how often a function is called
    function debounce(fn, delay) {
        let timeoutId;
        return function (...args) {
            clearTimeout(timeoutId);
            timeoutId = setTimeout(() => { fn.apply(this, args); }, delay);
        };
    }
    // Utility: Throttle function to limit function calls to once per interval
    function throttle(fn, limit) {
        let inThrottle = false;
        return function (...args) {
            if (!inThrottle) {
                fn.apply(this, args);
                inThrottle = true;
                setTimeout(() => inThrottle = false, limit);
            }
        };
    }
    // Initialize progress bar widths from data attributes (CSP-safe)
    document.querySelectorAll(".progress-bar[data-width]").forEach((bar) => {
        const width = bar.dataset["width"];
        if (width) {
            bar.style.width = `${width}%`;
        }
    });
    // Read configuration from data attributes
    const configEl = document.getElementById("dashboardConfig");
    if (!configEl)
        return;
    const searchItems = JSON.parse(configEl.dataset["searchItems"] ?? "[]");
    const goals = {
        calories: parseInt(configEl.dataset["goalCalories"] ?? "2000", 10),
        protein: parseFloat(configEl.dataset["goalProtein"] ?? "150"),
        carbs: parseFloat(configEl.dataset["goalCarbs"] ?? "250"),
        fat: parseFloat(configEl.dataset["goalFat"] ?? "65")
    };
    // Initialize Fuse.js with fuzzy search options
    const fuse = new Fuse(searchItems, {
        keys: ["name"],
        threshold: 0.4,
        distance: 100,
        includeScore: true,
        minMatchCharLength: 1,
        ignoreLocation: true
    });
    const itemSearch = document.getElementById("itemSearch");
    const searchResults = document.getElementById("searchResults");
    const foodIdInput = document.getElementById("foodIdInput");
    const ingredientIdInput = document.getElementById("ingredientIdInput");
    function selectItem(item) {
        if (!itemSearch || !searchResults || !foodIdInput || !ingredientIdInput)
            return;
        itemSearch.value = item.name;
        searchResults.classList.add("d-none");
        if (item.type === "food") {
            foodIdInput.value = String(item.id);
            ingredientIdInput.value = "";
        }
        else {
            ingredientIdInput.value = String(item.id);
            foodIdInput.value = "";
        }
    }
    function escapeHtml(text) {
        const div = document.createElement("div");
        div.textContent = text;
        return div.innerHTML;
    }
    if (itemSearch && searchResults) {
        // Debounced search handler to reduce Fuse.js runs on rapid typing
        const handleSearch = debounce(function () {
            const query = itemSearch.value.trim();
            if (query.length === 0) {
                searchResults.classList.add("d-none");
                searchResults.innerHTML = "";
                return;
            }
            const results = fuse.search(query).slice(0, 10);
            if (results.length === 0) {
                searchResults.innerHTML = '<div class="p-3 text-secondary">No matches found</div>';
                searchResults.classList.remove("d-none");
                return;
            }
            searchResults.innerHTML = results.map((result, index) => {
                const item = result.item;
                const icon = item.type === "food" ? "\u{1F37D}\u{FE0F}" : "\u{1F957}";
                const typeLabel = item.type === "food" ? "Food" : "Ingredient";
                const servingInfo = item.servingSize ? `/${item.servingSize}` : "";
                return `<div class="search-result p-2 border-bottom" data-index="${String(index)}">` +
                    '<div class="d-flex align-items-center gap-2">' +
                    `<span>${icon}</span>` +
                    '<div class="flex-grow-1">' +
                    `<div class="fw-medium">${escapeHtml(item.name)}</div>` +
                    `<small class="text-secondary">${typeLabel} \u00B7 ${String(item.calories)} kcal${servingInfo}</small>` +
                    '</div></div></div>';
            }).join("");
            searchResults.classList.remove("d-none");
            searchResults.dataset["results"] = JSON.stringify(results.map((r) => r.item));
        }, 150);
        itemSearch.addEventListener("input", handleSearch);
        // Handle result click
        searchResults.addEventListener("click", function (e) {
            const target = e.target;
            const resultEl = target.closest(".search-result");
            if (!resultEl)
                return;
            const indexStr = resultEl.dataset["index"];
            if (!indexStr)
                return;
            const index = parseInt(indexStr, 10);
            const resultsJson = this.dataset["results"];
            if (!resultsJson)
                return;
            const results = JSON.parse(resultsJson);
            const item = results[index];
            if (item) {
                selectItem(item);
            }
        });
        // Handle keyboard navigation
        itemSearch.addEventListener("keydown", function (e) {
            const items = searchResults.querySelectorAll(".search-result");
            const activeItem = searchResults.querySelector(".search-result.active");
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
            }
            else if (e.key === "ArrowUp") {
                e.preventDefault();
                activeIndex = Math.max(activeIndex - 1, 0);
                items.forEach((item, i) => {
                    item.classList.toggle("active", i === activeIndex);
                });
            }
            else if (e.key === "Enter" && activeIndex >= 0) {
                e.preventDefault();
                const resultsJson = searchResults.dataset["results"] ?? "[]";
                const results = JSON.parse(resultsJson);
                const item = results[activeIndex];
                if (item) {
                    selectItem(item);
                }
            }
            else if (e.key === "Escape") {
                searchResults.classList.add("d-none");
            }
        });
        // Close results when clicking outside
        document.addEventListener("click", function (e) {
            const target = e.target;
            if (!itemSearch.contains(target) && !searchResults.contains(target)) {
                searchResults.classList.add("d-none");
            }
        });
    }
    // Validate form before submit
    const addEntryForm = document.getElementById("addEntryForm");
    if (addEntryForm && foodIdInput && ingredientIdInput && itemSearch) {
        addEntryForm.addEventListener("submit", function (e) {
            if (!foodIdInput.value && !ingredientIdInput.value) {
                e.preventDefault();
                itemSearch.focus();
                showError("Please select a food or ingredient", "Use the search box to find and select an item before adding.");
            }
        });
    }
    // Show/hide floating summary based on scroll position
    const floatingSummary = document.getElementById("floatingSummary");
    const summaryHeader = document.querySelector(".summary-header");
    if (floatingSummary && summaryHeader) {
        function checkScroll() {
            const rect = summaryHeader.getBoundingClientRect();
            if (rect.top < 0) {
                floatingSummary.classList.remove("d-none");
            }
            else {
                floatingSummary.classList.add("d-none");
            }
        }
        // Throttled scroll handler to reduce reflow triggers
        window.addEventListener("scroll", throttle(checkScroll, 100), { passive: true });
        checkScroll();
    }
    // Toggle and servings functionality (requires entries)
    const entriesList = document.getElementById("entriesList");
    if (!entriesList)
        return;
    // Store previous values for animation
    let prevTotals = { cal: null, pro: null, carb: null, fat: null };
    function animatePop(element) {
        element.classList.remove("value-pop");
        void element.offsetWidth; // Trigger reflow
        element.classList.add("value-pop");
    }
    function updateTotals() {
        let totalCal = 0, totalPro = 0, totalCarb = 0, totalFat = 0;
        document.querySelectorAll(".entry-row").forEach((row) => {
            const checkbox = row.querySelector(".entry-toggle");
            const servingsInput = row.querySelector(".servings-input");
            const caloriesEl = row.querySelector(".entry-calories");
            const saveBtn = row.querySelector(".save-btn");
            if (!checkbox || !servingsInput || !caloriesEl || !saveBtn)
                return;
            const servings = parseFloat(servingsInput.value) || 0;
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
            // Show/hide save button if servings changed
            const original = parseFloat(row.dataset["originalServings"] ?? "0");
            if (Math.abs(servings - original) > 0.001) {
                saveBtn.classList.remove("d-none");
            }
            else {
                saveBtn.classList.add("d-none");
            }
        });
        // Check if values changed for animations
        const calChanged = prevTotals.cal !== null && prevTotals.cal !== totalCal;
        const proChanged = prevTotals.pro !== null && Math.round(prevTotals.pro) !== Math.round(totalPro);
        const carbChanged = prevTotals.carb !== null && Math.round(prevTotals.carb) !== Math.round(totalCarb);
        const fatChanged = prevTotals.fat !== null && Math.round(prevTotals.fat) !== Math.round(totalFat);
        // Update header displays
        const headerValues = document.querySelectorAll(".summary-header .fs-4.fw-bold");
        headerValues.forEach((el, i) => {
            if (i === 0 || i === 1) {
                el.textContent = String(totalCal);
                if (calChanged)
                    animatePop(el);
            }
        });
        // Update macro displays with animations
        const macroValues = document.querySelectorAll(".macro-progress .fw-bold");
        if (macroValues.length >= 3) {
            const carbEl = macroValues[0];
            const proEl = macroValues[1];
            const fatEl = macroValues[2];
            if (carbEl) {
                carbEl.textContent = String(Math.round(totalCarb));
                if (carbChanged)
                    animatePop(carbEl);
            }
            if (proEl) {
                proEl.textContent = String(Math.round(totalPro));
                if (proChanged)
                    animatePop(proEl);
            }
            if (fatEl) {
                fatEl.textContent = String(Math.round(totalFat));
                if (fatChanged)
                    animatePop(fatEl);
            }
        }
        // Update progress bars (CSS transition handles animation)
        const progressBars = document.querySelectorAll(".macro-progress .progress-bar");
        if (progressBars.length >= 3) {
            const carbBar = progressBars[0];
            const proBar = progressBars[1];
            const fatBar = progressBars[2];
            if (carbBar)
                carbBar.style.width = `${String(Math.min(100, (totalCarb / goals.carbs) * 100))}%`;
            if (proBar)
                proBar.style.width = `${String(Math.min(100, (totalPro / goals.protein) * 100))}%`;
            if (fatBar)
                fatBar.style.width = `${String(Math.min(100, (totalFat / goals.fat) * 100))}%`;
        }
        // Update calorie ring with animation
        const ringProgress = document.querySelector(".calorie-ring-progress");
        const ringSvg = document.querySelector(".calorie-ring-svg");
        if (ringProgress) {
            const pct = Math.min(100, (totalCal / goals.calories) * 100);
            ringProgress.setAttribute("stroke-dasharray", `${String(pct)}, 100`);
            if (calChanged && ringSvg) {
                ringSvg.classList.remove("ring-pulse");
                void ringSvg.offsetWidth;
                ringSvg.classList.add("ring-pulse");
            }
        }
        // Update floating summary with animations
        const floatCal = document.getElementById("floatCal");
        const floatPro = document.getElementById("floatPro");
        const floatCarb = document.getElementById("floatCarb");
        const floatFat = document.getElementById("floatFat");
        if (floatCal) {
            floatCal.textContent = String(totalCal);
            if (calChanged)
                animatePop(floatCal);
        }
        if (floatPro) {
            floatPro.textContent = String(Math.round(totalPro));
            if (proChanged)
                animatePop(floatPro);
        }
        if (floatCarb) {
            floatCarb.textContent = String(Math.round(totalCarb));
            if (carbChanged)
                animatePop(floatCarb);
        }
        if (floatFat) {
            floatFat.textContent = String(Math.round(totalFat));
            if (fatChanged)
                animatePop(floatFat);
        }
        // Store current values for next comparison
        prevTotals = { cal: totalCal, pro: totalPro, carb: totalCarb, fat: totalFat };
    }
    entriesList.addEventListener("change", function (e) {
        const target = e.target;
        if (target.classList.contains("entry-toggle")) {
            updateTotals();
        }
    });
    entriesList.addEventListener("input", function (e) {
        const target = e.target;
        if (target.classList.contains("servings-input")) {
            updateTotals();
        }
    });
    // Save button click - update entry servings
    entriesList.addEventListener("click", function (e) {
        const target = e.target;
        if (target.classList.contains("save-btn")) {
            const row = target.closest(".entry-row");
            if (!row)
                return;
            const entryId = row.dataset["id"];
            const servingsInput = row.querySelector(".servings-input");
            if (!entryId || !servingsInput)
                return;
            const servings = servingsInput.value;
            // Send update request
            const formData = new FormData();
            formData.append("servings", servings);
            fetch(`/entries/${entryId}/servings`, {
                method: "POST",
                body: formData
            }).then((response) => {
                if (response.ok) {
                    row.dataset["originalServings"] = servings;
                    target.classList.add("d-none");
                }
            }).catch(() => {
                // Error handled by global fetch wrapper
            });
        }
    });
})();
