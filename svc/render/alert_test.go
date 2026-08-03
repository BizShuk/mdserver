package render

import (
	"strings"
	"testing"
)

// renderString is the shared helper for the render package's tests.
func renderString(t *testing.T, source string) string {
	t.Helper()
	doc, err := New().Render([]byte(source))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return string(doc.HTML)
}

func TestAlertTypes(t *testing.T) {
	for _, alertType := range []string{"note", "tip", "important", "warning", "caution"} {
		t.Run(alertType, func(t *testing.T) {
			marker := strings.ToUpper(alertType)
			html := renderString(t, "> [!"+marker+"]\n> Body text here.\n")

			if !strings.Contains(html, `md-alert md-alert-`+alertType) {
				t.Errorf("missing alert class for %s:\n%s", alertType, html)
			}
			// The marker is what the reader must never see; it is replaced by
			// the styled title.
			if strings.Contains(html, "[!"+marker+"]") {
				t.Errorf("marker %q leaked into output:\n%s", "[!"+marker+"]", html)
			}
			if !strings.Contains(html, "Body text here.") {
				t.Errorf("alert body was dropped:\n%s", html)
			}
			if strings.Contains(html, "<blockquote>") {
				t.Errorf("alert should replace the blockquote:\n%s", html)
			}
		})
	}
}

func TestAlertMarkerOnlyWithNoBody(t *testing.T) {
	html := renderString(t, "> [!NOTE]\n")
	if strings.Contains(html, "[!NOTE]") {
		t.Errorf("marker leaked into output:\n%s", html)
	}
	if !strings.Contains(html, "md-alert-note") {
		t.Errorf("expected a note alert:\n%s", html)
	}
}

func TestAlertMultiParagraphBody(t *testing.T) {
	html := renderString(t, "> [!WARNING]\n> First paragraph.\n>\n> Second paragraph.\n")
	if strings.Contains(html, "[!WARNING]") {
		t.Errorf("marker leaked into output:\n%s", html)
	}
	for _, want := range []string{"First paragraph.", "Second paragraph."} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q:\n%s", want, html)
		}
	}
}

func TestAlertInlineMarkupInBodySurvives(t *testing.T) {
	html := renderString(t, "> [!TIP]\n> Use `go build` and **bold** text.\n")
	if strings.Contains(html, "[!TIP]") {
		t.Errorf("marker leaked into output:\n%s", html)
	}
	for _, want := range []string{"<code>go build</code>", "<strong>bold</strong>"} {
		if !strings.Contains(html, want) {
			t.Errorf("inline markup lost, missing %q:\n%s", want, html)
		}
	}
}

func TestPlainBlockquoteIsNotAnAlert(t *testing.T) {
	html := renderString(t, "> just a quote\n> [!NOT_A_TYPE]\n")
	if strings.Contains(html, "md-alert") {
		t.Errorf("plain blockquote became an alert:\n%s", html)
	}
	if !strings.Contains(html, "<blockquote>") {
		t.Errorf("expected a blockquote:\n%s", html)
	}
	// An unknown marker is ordinary text and must be preserved verbatim.
	if !strings.Contains(html, "[!NOT_A_TYPE]") {
		t.Errorf("unknown marker text was dropped:\n%s", html)
	}
}

func TestLowercaseMarkerIsNotAnAlert(t *testing.T) {
	// GitHub only recognises the uppercase form.
	html := renderString(t, "> [!note]\n> body\n")
	if strings.Contains(html, "md-alert") {
		t.Errorf("lowercase marker should not create an alert:\n%s", html)
	}
}
