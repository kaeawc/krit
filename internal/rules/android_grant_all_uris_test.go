package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
)

func TestGrantAllUris_Manifest(t *testing.T) {
	r := findManifestRule(t, "GrantAllUris")
	run := func(t *testing.T, body string) (lines []int) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "src", "main", "AndroidManifest.xml")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, f := range runManifestRule(r, &rules.Manifest{Path: path}) {
			if f.Message != "Content provider shares everything; this is potentially dangerous" {
				t.Fatalf("unexpected message %q", f.Message)
			}
			lines = append(lines, f.Line)
		}
		return lines
	}
	const header = `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application>
        <provider android:name=".P" android:authorities="a" android:exported="false">
`
	const footer = `        </provider>
    </application>
</manifest>`

	t.Run("path, pathPrefix and pathPattern equal to a slash are reported", func(t *testing.T) {
		got := run(t, header+`            <grant-uri-permission android:path="/" />
            <grant-uri-permission android:pathPrefix="/" />
            <grant-uri-permission android:pathPattern="/" />
`+footer)
		if len(got) != 3 || got[0] != 4 || got[1] != 5 || got[2] != 6 {
			t.Fatalf("expected findings on lines 4, 5, 6, got %v", got)
		}
	})
	t.Run("narrower values are clean", func(t *testing.T) {
		got := run(t, header+`            <grant-uri-permission android:path="/a" />
            <grant-uri-permission android:pathPrefix="/a" />
            <grant-uri-permission android:pathPattern=".*" />
            <grant-uri-permission android:pathPattern="/.*" />
            <grant-uri-permission android:pathPrefix="" />
`+footer)
		if len(got) != 0 {
			t.Fatalf("expected no findings, got %v", got)
		}
	})
	t.Run("an attribute on a later line is reported on its own line", func(t *testing.T) {
		got := run(t, header+`            <grant-uri-permission
                android:path="/" />
`+footer)
		if len(got) != 1 || got[0] != 5 {
			t.Fatalf("expected a finding on line 5, got %v", got)
		}
	})
	t.Run("manifest without grant-uri-permission is clean", func(t *testing.T) {
		if got := run(t, header+footer); len(got) != 0 {
			t.Fatalf("expected no findings, got %v", got)
		}
	})
}
