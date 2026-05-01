// Dashboard page JavaScript
(function() {
    "use strict";

    // Initialize progress bar widths from data attributes (CSP-safe)
    document.querySelectorAll(".progress-bar[data-width]").forEach(function(bar) {
        bar.style.width = bar.dataset.width + "%";
    });

    // Read configuration from data attributes
    const configEl = document.getElementById("dashboardConfig");
    if (!configEl) return;

    const searchItems = JSON.parse(configEl.dataset.searchItems || "[]");
    const goals = {
        calories: parseInt(configEl.dataset.goalCalories, 10) || 2000,
        protein: parseFloat(configEl.dataset.goalProtein) || 150,
        carbs: parseFloat(configEl.dataset.goalCarbs) || 250,
        fat: parseFloat(configEl.dataset.goalFat) || 65
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

    if (itemSearch) {
        itemSearch.addEventListener("input", function() {
            const query = this.value.trim();

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

            searchResults.innerHTML = results.map(function(result, index) {
                const item = result.item;
                const icon = item.type === "food" ? "\u{1F37D}\u{FE0F}" : "\u{1F957}";
                const typeLabel = item.type === "food" ? "Food" : "Ingredient";
                return '<div class="search-result p-2 border-bottom" data-index="' + index + '" style="cursor: pointer;">' +
                    '<div class="d-flex align-items-center gap-2">' +
                    '<span>' + icon + '</span>' +
                    '<div class="flex-grow-1">' +
                    '<div class="fw-medium">' + item.name + '</div>' +
                    '<small class="text-secondary">' + typeLabel + ' \u00B7 ' + item.calories + ' kcal' + (item.servingSize ? '/' + item.servingSize : '') + '</small>' +
                    '</div></div></div>';
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
            const item = results[index];

            selectItem(item);
        });

        // Handle keyboard navigation
        itemSearch.addEventListener("keydown", function(e) {
            const items = searchResults.querySelectorAll(".search-result");
            const activeItem = searchResults.querySelector(".search-result.active");
            let activeIndex = activeItem ? parseInt(activeItem.dataset.index, 10) : -1;

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
                    selectItem(results[activeIndex]);
                }
            } else if (e.key === "Escape") {
                searchResults.classList.add("d-none");
            }
        });

        // Close results when clicking outside
        document.addEventListener("click", function(e) {
            if (!itemSearch.contains(e.target) && !searchResults.contains(e.target)) {
                searchResults.classList.add("d-none");
            }
        });
    }

    function selectItem(item) {
        itemSearch.value = item.name;
        searchResults.classList.add("d-none");

        if (item.type === "food") {
            foodIdInput.value = item.id;
            ingredientIdInput.value = "";
        } else {
            ingredientIdInput.value = item.id;
            foodIdInput.value = "";
        }
    }

    // Validate form before submit
    const addEntryForm = document.getElementById("addEntryForm");
    if (addEntryForm) {
        addEntryForm.addEventListener("submit", function(e) {
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
            } else {
                floatingSummary.classList.add("d-none");
            }
        }

        window.addEventListener("scroll", checkScroll, { passive: true });
        checkScroll();
    }

    // Toggle and servings functionality (requires entries)
    const entriesList = document.getElementById("entriesList");
    if (!entriesList) return;

    // Store previous values for animation
    let prevTotals = { cal: null, pro: null, carb: null, fat: null };

    function animatePop(element) {
        element.classList.remove("value-pop");
        void element.offsetWidth; // Trigger reflow
        element.classList.add("value-pop");
    }

    function updateTotals() {
        let totalCal = 0, totalPro = 0, totalCarb = 0, totalFat = 0;

        document.querySelectorAll(".entry-row").forEach(function(row) {
            const checkbox = row.querySelector(".entry-toggle");
            const servingsInput = row.querySelector(".servings-input");
            const servings = parseFloat(servingsInput.value) || 0;

            // Calculate this row's nutrition
            const baseCal = parseInt(row.dataset.baseCalories, 10) || 0;
            const basePro = parseFloat(row.dataset.baseProtein) || 0;
            const baseCarb = parseFloat(row.dataset.baseCarbs) || 0;
            const baseFat = parseFloat(row.dataset.baseFat) || 0;

            const rowCal = Math.round(baseCal * servings);
            const rowPro = basePro * servings;
            const rowCarb = baseCarb * servings;
            const rowFat = baseFat * servings;

            // Update row's calorie display
            row.querySelector(".entry-calories").textContent = rowCal + " kcal";

            if (checkbox.checked) {
                totalCal += rowCal;
                totalPro += rowPro;
                totalCarb += rowCarb;
                totalFat += rowFat;
            }

            // Visual feedback for disabled items
            row.classList.toggle("disabled", !checkbox.checked);

            // Show/hide save button if servings changed
            const saveBtn = row.querySelector(".save-btn");
            const original = parseFloat(row.dataset.originalServings) || 0;
            if (Math.abs(servings - original) > 0.001) {
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

        // Update header displays
        const headerValues = document.querySelectorAll(".summary-header .fs-4.fw-bold");
        headerValues.forEach(function(el, i) {
            if (i === 0 || i === 1) {
                el.textContent = totalCal;
                if (calChanged) animatePop(el);
            }
        });

        // Update macro displays with animations
        const macroValues = document.querySelectorAll(".macro-progress .fw-bold");
        if (macroValues.length >= 3) {
            macroValues[0].textContent = Math.round(totalCarb);
            macroValues[1].textContent = Math.round(totalPro);
            macroValues[2].textContent = Math.round(totalFat);

            if (carbChanged) animatePop(macroValues[0]);
            if (proChanged) animatePop(macroValues[1]);
            if (fatChanged) animatePop(macroValues[2]);
        }

        // Update progress bars (CSS transition handles animation)
        const progressBars = document.querySelectorAll(".macro-progress .progress-bar");
        if (progressBars.length >= 3) {
            progressBars[0].style.width = Math.min(100, (totalCarb / goals.carbs) * 100) + "%";
            progressBars[1].style.width = Math.min(100, (totalPro / goals.protein) * 100) + "%";
            progressBars[2].style.width = Math.min(100, (totalFat / goals.fat) * 100) + "%";
        }

        // Update calorie ring with animation
        const ringProgress = document.querySelector(".calorie-ring-progress");
        const ringSvg = document.querySelector(".calorie-ring-svg");
        if (ringProgress) {
            const pct = Math.min(100, (totalCal / goals.calories) * 100);
            ringProgress.setAttribute("stroke-dasharray", pct + ", 100");

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
            floatCal.textContent = totalCal;
            if (calChanged) animatePop(floatCal);
        }
        if (floatPro) {
            floatPro.textContent = Math.round(totalPro);
            if (proChanged) animatePop(floatPro);
        }
        if (floatCarb) {
            floatCarb.textContent = Math.round(totalCarb);
            if (carbChanged) animatePop(floatCarb);
        }
        if (floatFat) {
            floatFat.textContent = Math.round(totalFat);
            if (fatChanged) animatePop(floatFat);
        }

        // Store current values for next comparison
        prevTotals = { cal: totalCal, pro: totalPro, carb: totalCarb, fat: totalFat };
    }

    entriesList.addEventListener("change", function(e) {
        if (e.target.classList.contains("entry-toggle")) {
            updateTotals();
        }
    });

    entriesList.addEventListener("input", function(e) {
        if (e.target.classList.contains("servings-input")) {
            updateTotals();
        }
    });

    // Save button click - update entry servings
    entriesList.addEventListener("click", function(e) {
        if (e.target.classList.contains("save-btn")) {
            const row = e.target.closest(".entry-row");
            const entryId = row.dataset.id;
            const servings = row.querySelector(".servings-input").value;

            // Send update request
            const formData = new FormData();
            formData.append("servings", servings);

            fetch("/entries/" + entryId + "/servings", {
                method: "POST",
                body: formData
            }).then(function(response) {
                if (response.ok) {
                    row.dataset.originalServings = servings;
                    e.target.classList.add("d-none");
                }
            });
        }
    });
})();
