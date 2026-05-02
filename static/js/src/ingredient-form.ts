// Ingredient form TypeScript

(function(): void {
    "use strict";

    const unitField = document.getElementById("unit_name_field");
    const servingSizeInput = document.getElementById("serving_size") as HTMLInputElement | null;
    const weightRadio = document.getElementById("serving_type_weight") as HTMLInputElement | null;
    const unitRadio = document.getElementById("serving_type_unit") as HTMLInputElement | null;

    if (!unitField || !servingSizeInput || !weightRadio || !unitRadio) return;

    function toggleUnitField(): void {
        if (!unitField || !servingSizeInput || !unitRadio) return;

        if (unitRadio.checked) {
            unitField.classList.remove("d-none");
        } else {
            unitField.classList.add("d-none");
            servingSizeInput.value = "";
        }
    }

    weightRadio.addEventListener("change", toggleUnitField);
    unitRadio.addEventListener("change", toggleUnitField);
})();
