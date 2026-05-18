package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

func TestStringContainsHtmlWithoutCDATA(t *testing.T) {
	r := findResourceRule(t, "StringContainsHtmlWithoutCDATA")

	write := func(t *testing.T, path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
	}
	scan := func(t *testing.T, root string) *android.ResourceIndex {
		t.Helper()
		idx, err := android.ScanResourceDir(root)
		if err != nil {
			t.Fatalf("ScanResourceDir(%s): %v", root, err)
		}
		return idx
	}

	t.Run("literal anchor tag triggers", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"html_msg\">Click <a href=\"x\">here</a></string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if !strings.Contains(findings[0].Message, "html_msg") {
			t.Fatalf("unexpected message: %q", findings[0].Message)
		}
	})

	t.Run("CDATA-wrapped is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"html_msg\"><![CDATA[Click <a href=\"x\">here</a>]]></string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("entity-escaped is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"html_msg\">Click &lt;a href=\"x\"&gt;here&lt;/a&gt;</string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("plain text is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"hello\">Hello world</string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("inline bold tag triggers", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"emphasis\">Be <b>bold</b></string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})

	t.Run("locale variant triggers", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"hello\">Hello</string></resources>\n")
		write(t, filepath.Join(resDir, "values-fr", "strings.xml"),
			"<resources><string name=\"hello\">Cliquez <a href=\"x\">ici</a></string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if !strings.Contains(findings[0].File, "values-fr") {
			t.Fatalf("expected finding in values-fr, got: %s", findings[0].File)
		}
	})

	// Regression: xliff:g placeholders are the canonical Android i18n shape
	// and must not be flagged. The original implementation fired on every
	// <string> that had any child element.
	t.Run("xliff:g placeholder is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources xmlns:xliff=\"urn:oasis:names:tc:xliff:document:1.2\">"+
				"<string name=\"played\">Hello <xliff:g id=\"num\" example=\"3\">%d</xliff:g> times</string>"+
				"</resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for xliff:g placeholder, got %d: %v", len(findings), findings)
		}
	})

	// Regression: <annotation> spans are an Android-supported markup primitive
	// (SpannedString.Annotation) and must not be flagged.
	t.Run("annotation span is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"title\">Welcome <annotation font=\"title_emphasis\">friend</annotation>!</string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for <annotation>, got %d: %v", len(findings), findings)
		}
	})

	// Mixing xliff placeholder with real unescaped HTML markup should still
	// trigger — the rule should look at the non-placeholder children.
	t.Run("xliff:g plus inline HTML still triggers", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources xmlns:xliff=\"urn:oasis:names:tc:xliff:document:1.2\">"+
				"<string name=\"mixed\">Won <xliff:g id=\"n\">%d</xliff:g> <b>games</b></string>"+
				"</resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for xliff + <b>, got %d: %v", len(findings), findings)
		}
	})
}
