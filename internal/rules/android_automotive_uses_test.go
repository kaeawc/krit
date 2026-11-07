package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func runInvalidUsesTag(t *testing.T, files map[string]string) []scanner.Finding {
	t.Helper()
	dir := t.TempDir()
	files["layout/main.xml"] = `<FrameLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent"/>`
	for name, body := range files {
		path := filepath.Join(dir, "res", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := android.ScanResourceDir(filepath.Join(dir, "res"))
	if err != nil {
		t.Fatal(err)
	}
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "InvalidUsesTagAttribute" {
			rule = r
		}
	}
	if rule == nil {
		t.Fatal("InvalidUsesTagAttribute not registered")
	}
	return runResourceRule(rule, idx)
}

func TestInvalidUsesTagAttribute_Automotive(t *testing.T) {
	const message = "Expecting one of `media`, `notification`, `sms`, or `template` for the name attribute in uses tag"
	t.Run("only the four names are accepted, on the name attribute", func(t *testing.T) {
		findings := runInvalidUsesTag(t, map[string]string{"xml/automotive_app_desc.xml": `<automotiveApp>
    <uses name="media" />
    <uses name="Media" />
    <uses name="video" />
    <uses name="sms"/>
    <uses name="notification"/>
    <uses name="template"/>
    <uses />
    <uses
        name="bogus" />
</automotiveApp>
`})
		var lines []int
		for _, f := range findings {
			if f.Message != message {
				t.Fatalf("unexpected message %q", f.Message)
			}
			lines = append(lines, f.Line)
		}
		if len(lines) != 3 || lines[0] != 3 || lines[1] != 4 || lines[2] != 10 {
			t.Fatalf("expected findings on lines 3, 4 and 10, got %v", lines)
		}
	})
	t.Run("any xml resource folder with an automotiveApp root is checked", func(t *testing.T) {
		findings := runInvalidUsesTag(t, map[string]string{
			"xml/other.xml":         `<automotiveApp><uses name="nope"/></automotiveApp>`,
			"xml-v21/other.xml":     `<automotiveApp><uses name="nope"/></automotiveApp>`,
			"xml/unrelated.xml":     `<paths><uses name="nope"/></paths>`,
			"values/automotive.xml": `<automotiveApp><uses name="nope"/></automotiveApp>`,
		})
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})
}
