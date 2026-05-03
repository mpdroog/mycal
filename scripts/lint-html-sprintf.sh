#!/bin/bash
# Lint Go files for unsafe fmt.Sprintf with HTML content
# Use safehtml.Sprintf instead for auto-escaping
HANDLERS_DIR="handlers"
ERRORS=0

echo "Checking for unsafe HTML in fmt.Sprintf..."

# Look for fmt.Sprintf with HTML tags (< or >) in the format string
# Exclude test files
if grep -rn 'fmt\.Sprintf.*[<>]' "$HANDLERS_DIR"/*.go 2>/dev/null | grep -v '_test\.go'; then
    echo ""
    echo "^^^ ERROR: fmt.Sprintf with HTML detected"
    echo "    Use safehtml.Sprintf instead for automatic HTML escaping"
    echo "    Example: safehtml.Sprintf(\"<div>%s</div>\", userInput)"
    ERRORS=1
fi

# Also check for fmt.Fprintf with HTML (writing directly to response)
if grep -rn 'fmt\.Fprintf.*[<>]' "$HANDLERS_DIR"/*.go 2>/dev/null | grep -v '_test\.go'; then
    echo ""
    echo "^^^ ERROR: fmt.Fprintf with HTML detected"
    echo "    Use safehtml.Sprintf and w.Write instead"
    ERRORS=1
fi

if [ $ERRORS -eq 0 ]; then
    echo "OK - No unsafe HTML sprintf found"
fi

exit $ERRORS
