// Global TypeScript - Error handling, fetch wrapper, toast functionality

declare const bootstrap: {
    Modal: new (element: Element) => { show: () => void };
    Toast: new (element: Element) => { show: () => void };
};

interface PendingError {
    message: string;
    details: string;
}

interface WindowWithErrors extends Window {
    _pendingErrors?: PendingError[];
    showError?: (message: string, details: string) => void;
}

(function(): void {
    "use strict";

    const win = window as WindowWithErrors;

    // Initialize pending errors array for early errors
    win._pendingErrors = win._pendingErrors || [];

    function getErrorStack(reason: unknown): string {
        if (reason && typeof reason === "object" && "stack" in reason) {
            return String(reason.stack);
        }
        return String(reason);
    }

    function formatMessage(msg: string | Event): string {
        return typeof msg === "string" ? msg : "Error event";
    }

    // Early error handlers (before Bootstrap loads, these capture errors)
    window.onerror = function(message, source, lineno, colno, error): boolean {
        const details = [
            "Error: " + formatMessage(message),
            "Source: " + String(source),
            "Line: " + String(lineno) + ", Column: " + String(colno),
            error && error.stack ? "\nStack:\n" + error.stack : ""
        ].join("\n");
        win._pendingErrors!.push({ message: "A JavaScript error occurred.", details: details });
        return false;
    };

    window.onunhandledrejection = function(event: PromiseRejectionEvent): void {
        const details = event.reason ? getErrorStack(event.reason) : "Unknown promise rejection";
        win._pendingErrors!.push({ message: "An async operation failed.", details: details });
    };

    // Wait for DOM to be ready
    document.addEventListener("DOMContentLoaded", function(): void {
        const errorModalEl = document.getElementById("errorModal");
        const errorMessage = document.getElementById("errorMessage");
        const errorDetails = document.getElementById("errorDetails");
        const reloadBtn = document.getElementById("reloadPageBtn");

        if (!errorModalEl || !errorMessage || !errorDetails) return;

        const errorModal = new bootstrap.Modal(errorModalEl);

        // Global showError function
        win.showError = function(msg: string, det: string): void {
            errorMessage.textContent = msg || "An unexpected error occurred.";
            errorDetails.textContent = det || "No additional details available.";
            errorModal.show();
        };

        // Show any errors that occurred before this script loaded
        if (win._pendingErrors && win._pendingErrors.length > 0 && win._pendingErrors[0]) {
            const err = win._pendingErrors[0];
            win.showError(err.message, err.details);
        }

        // Replace early error handlers with full versions
        window.onerror = function(message, source, lineno, colno, error): boolean {
            const det = [
                "Error: " + formatMessage(message),
                "Source: " + String(source),
                "Line: " + String(lineno) + ", Column: " + String(colno),
                error && error.stack ? "\nStack:\n" + error.stack : ""
            ].join("\n");
            win.showError!("A JavaScript error occurred.", det);
            return false;
        };

        window.onunhandledrejection = function(event: PromiseRejectionEvent): void {
            const det = event.reason ? getErrorStack(event.reason) : "Unknown promise rejection";
            win.showError!("An async operation failed.", det);
        };

        // Override global fetch to add error handling
        const originalFetch = window.fetch;
        window.fetch = async function(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
            const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
            try {
                const response = await originalFetch(input, init);

                if (!response.ok) {
                    const text = await response.clone().text();
                    const det = [
                        "URL: " + url,
                        "Status: " + String(response.status) + " " + response.statusText,
                        "Response: " + text
                    ].join("\n");
                    win.showError!("Request failed: " + response.statusText, det);
                }

                return response;
            } catch (err) {
                const error = err as Error;
                const det = [
                    "URL: " + url,
                    "Error: " + error.message,
                    error.stack ? "\nStack:\n" + error.stack : ""
                ].join("\n");
                win.showError!("Network error: Could not reach server.", det);
                throw err;
            }
        };

        // Reload page button
        if (reloadBtn) {
            reloadBtn.addEventListener("click", function(): void {
                location.reload();
            });
        }

        // Toast/undo functionality
        const params = new URLSearchParams(window.location.search);
        const deleted = params.get("deleted");
        const id = params.get("id");

        if (deleted && id) {
            const toast = document.getElementById("undo-toast");
            const toastMessage = document.getElementById("toast-message");
            const undoForm = document.getElementById("undo-form") as HTMLFormElement | null;

            if (toast && toastMessage && undoForm) {
                // Set message based on entity type
                const entityNames: Record<string, string> = {
                    "entry": "Entry",
                    "ingredient": "Ingredient",
                    "food": "Food"
                };
                toastMessage.textContent = (entityNames[deleted] || "Item") + " deleted";

                // Set restore endpoint
                const endpoints: Record<string, string> = {
                    "entry": "/entries/" + id + "/restore",
                    "ingredient": "/ingredients/" + id + "/restore",
                    "food": "/foods/" + id + "/restore"
                };
                undoForm.action = endpoints[deleted] || "#";

                // Show toast
                const bsToast = new bootstrap.Toast(toast);
                bsToast.show();

                // Clean URL without reload
                params.delete("deleted");
                params.delete("id");
                let cleanUrl = window.location.pathname;
                if (params.toString()) {
                    cleanUrl += "?" + params.toString();
                }
                window.history.replaceState({}, "", cleanUrl);
            }
        }
    });
})();
