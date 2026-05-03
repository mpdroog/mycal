package safehtml

import "testing"

func TestSprintfEscapesStrings(t *testing.T) {
	input := "<script>alert('xss')</script>"
	result := Sprintf("<div>%s</div>", input)
	expected := "<div>&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;</div>"

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestSprintfPreservesUnsafeHTML(t *testing.T) {
	trusted := UnsafeHTML("<strong>bold</strong>")
	result := Sprintf("<div>%s</div>", trusted)
	expected := "<div><strong>bold</strong></div>"

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestSprintfMixedArgs(t *testing.T) {
	userInput := "<em>user</em>"
	trusted := UnsafeHTML("<b>safe</b>")
	count := 42

	result := Sprintf("<p>%s %s %d</p>", userInput, trusted, count)
	expected := "<p>&lt;em&gt;user&lt;/em&gt; <b>safe</b> 42</p>"

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestSprintfNumbers(t *testing.T) {
	result := Sprintf("<span>%d %.1f</span>", 100, 3.14)
	expected := "<span>100 3.1</span>"

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestSprintfAttributes(t *testing.T) {
	// Common XSS in attributes: " onclick="alert(1)
	malicious := `" onclick="alert(1)`
	result := Sprintf(`<input value="%s">`, malicious)
	expected := `<input value="&#34; onclick=&#34;alert(1)">`

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestEscape(t *testing.T) {
	input := `<a href="test">link & stuff</a>`
	result := Escape(input)
	expected := `&lt;a href=&#34;test&#34;&gt;link &amp; stuff&lt;/a&gt;`

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestJoin(t *testing.T) {
	elems := []string{"<one>", "two", "<three>"}
	result := Join(elems, ", ")
	expected := UnsafeHTML("&lt;one&gt;, two, &lt;three&gt;")

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}
