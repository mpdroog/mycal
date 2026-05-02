#!/bin/bash
# Lint HTML templates for CSP violations
TEMPLATES_DIR="templates"
ERRORS=0

echo "Checking for CSP violations..."

# Check for inline event handlers (onclick, onchange, onload, etc.)
if grep -En ' on[a-z]+="' "$TEMPLATES_DIR"/*.html 2>/dev/null; then
    echo "^^^ ERROR: Inline event handlers (CSP violation)"
    ERRORS=1
fi

# Check for inline style attributes
if grep -En ' style="' "$TEMPLATES_DIR"/*.html 2>/dev/null; then
    echo "^^^ ERROR: Inline style attributes (CSP violation)"
    ERRORS=1
fi

# Check for inline scripts (script tags without src, excluding type="application/json")
if grep -En '<script>' "$TEMPLATES_DIR"/*.html 2>/dev/null; then
    echo "^^^ ERROR: Inline scripts (CSP violation)"
    ERRORS=1
fi

# Check for inline style blocks
if grep -En '<style>' "$TEMPLATES_DIR"/*.html 2>/dev/null; then
    echo "^^^ ERROR: Inline style blocks (CSP violation)"
    ERRORS=1
fi

if [ $ERRORS -eq 0 ]; then
    echo "OK - No CSP violations found"
fi

exit $ERRORS
