package rules_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// runRtlCompat writes a module (manifest plus layouts by relative path) and runs RtlCompatResource over it.
func runRtlCompat(t *testing.T, manifest string, layouts map[string]string) []scanner.Finding {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AndroidManifest.xml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, body := range layouts {
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
		if r.ID == "RtlCompatResource" {
			rule = r
		}
	}
	if rule == nil {
		t.Fatal("RtlCompatResource not registered")
	}
	collector := scanner.NewFindingCollector(0)
	ctx := &api.Context{
		ResourceIndex: idx, Rule: rule, Collector: collector, LibraryFacts: librarymodel.DefaultFacts(),
		RtlMainsFor: rtlMainsFromManifest(filepath.Join(dir, "AndroidManifest.xml")),
	}
	rule.Check(ctx)
	return api.ContextFindings(ctx)
}

// rtlMainsFromManifest answers RtlMainsFor with the app the manifest describes, as the pipeline does.
func rtlMainsFromManifest(path string) func(string) ([]api.RtlMainProject, bool) {
	parsed, err := android.ParseManifest(path)
	if err != nil {
		return nil
	}
	m := android.ConvertManifest(parsed, path)
	levels := android.ResolveSDKLevels(path)
	main := api.RtlMainProject{
		HasApplication: m.HasApplication || m.AppLine > 0,
		RtlOff:         m.SupportsRtl != nil && !*m.SupportsRtl,
		MinSdk:         levels.MinSdk,
		TargetSdk:      levels.TargetSdk,
	}
	return func(string) ([]api.RtlMainProject, bool) { return []api.RtlMainProject{main}, true }
}

func rtlCompatManifest(min, target, application string) string {
	return `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="test">
    <uses-sdk android:minSdkVersion="` + min + `" android:targetSdkVersion="` + target + `"/>
    <application` + application + `/>
</manifest>`
}

const rtlCompatLayoutHead = `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:tools="http://schemas.android.com/tools"
    android:layout_width="match_parent" android:layout_height="match_parent">
`
const rtlCompatLayoutTail = "</LinearLayout>\n"

func rtlCompatLines(findings []scanner.Finding) []int {
	var lines []int
	for _, f := range findings {
		lines = append(lines, f.Line)
	}
	sort.Ints(lines)
	return lines
}

func TestRtlCompatResource(t *testing.T) {
	layout := func(body string) map[string]string {
		return map[string]string{"layout/main.xml": rtlCompatLayoutHead + body + rtlCompatLayoutTail}
	}
	pre17 := rtlCompatManifest("14", "21", ` android:supportsRtl="true"`)

	t.Run("textAlignment without gravity needs a gravity before API 17", func(t *testing.T) {
		findings := runRtlCompat(t, pre17, layout(`    <TextView android:textAlignment="viewStart" />
    <TextView android:textAlignment="center" />
    <TextView android:textAlignment="gravity" />
`))
		if got := rtlCompatLines(findings); len(got) != 2 || got[0] != 4 || got[1] != 5 {
			t.Fatalf("expected findings on lines 4 and 5, got %v", got)
		}
		want := "To support older versions than API 17 (project specifies 14) you must **also** specify `gravity` or `layout_gravity=\"start\"`"
		if findings[0].Message != want {
			t.Fatalf("unexpected message %q", findings[0].Message)
		}
	})
	t.Run("an existing gravity must agree with textAlignment", func(t *testing.T) {
		findings := runRtlCompat(t, pre17, layout(`    <TextView android:textAlignment="viewStart" android:gravity="start|bottom" />
    <TextView android:textAlignment="viewStart" android:gravity="left" />
    <TextView android:textAlignment="viewEnd" android:gravity="start" android:layout_gravity="end" />
    <TextView android:textAlignment="center" android:gravity="center" />
    <TextView android:textAlignment="textStart" android:layout_gravity="start" />
`))
		if got := rtlCompatLines(findings); len(got) != 2 || got[0] != 5 || got[1] != 6 {
			t.Fatalf("expected findings on lines 5 and 6, got %v", got)
		}
		want := "Inconsistent alignment specification between `textAlignment` and `gravity` attributes: was `left`, expected `start`"
		if findings[0].Message != want {
			t.Fatalf("unexpected message %q", findings[0].Message)
		}
	})
	t.Run("start and end attributes need their left and right twins before API 17", func(t *testing.T) {
		findings := runRtlCompat(t, pre17, layout(`    <TextView android:paddingStart="4dp" />
    <TextView android:paddingStart="4dp" android:paddingLeft="4dp" />
    <TextView android:paddingStart="4dp" android:paddingEnd="4dp" />
    <TextView android:layout_marginEnd="2dp" />
    <TextView android:drawableStart="@null" />
`))
		if got := rtlCompatLines(findings); len(got) != 3 || got[0] != 4 || got[1] != 7 || got[2] != 8 {
			t.Fatalf("expected findings on lines 4, 7 and 8, got %v", got)
		}
		want := "To support older versions than API 17 (project specifies 14) you should **also** add `android:paddingLeft=\"4dp\"`"
		if findings[0].Message != want {
			t.Fatalf("unexpected message %q", findings[0].Message)
		}
	})
	t.Run("API 17 and later needs only consistency", func(t *testing.T) {
		findings := runRtlCompat(t, rtlCompatManifest("17", "21", ""), layout(`    <TextView android:textAlignment="viewStart" />
    <TextView android:paddingStart="4dp" />
    <TextView android:textAlignment="viewStart" android:gravity="left" />
`))
		if got := rtlCompatLines(findings); len(got) != 1 || got[0] != 6 {
			t.Fatalf("expected one finding on line 6, got %v", got)
		}
	})
	t.Run("a v17 resource folder needs no compat attributes", func(t *testing.T) {
		findings := runRtlCompat(t, pre17, map[string]string{
			"layout-v17/main.xml": rtlCompatLayoutHead + `    <TextView android:textAlignment="viewStart" />
    <TextView android:paddingStart="4dp" />
` + rtlCompatLayoutTail,
		})
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", rtlCompatLines(findings))
		}
	})
	t.Run("RTL is on unless supportsRtl is false", func(t *testing.T) {
		body := layout(`    <TextView android:paddingStart="4dp" />
`)
		if got := len(runRtlCompat(t, rtlCompatManifest("14", "21", ""), body)); got != 1 {
			t.Errorf("supportsRtl absent: expected 1 finding, got %d", got)
		}
		if got := len(runRtlCompat(t, rtlCompatManifest("14", "21", ` android:supportsRtl="false"`), body)); got != 0 {
			t.Errorf("supportsRtl=false: expected 0 findings, got %d", got)
		}
	})
	t.Run("targeting below 17 or undeclared SDK levels reports nothing", func(t *testing.T) {
		body := layout(`    <TextView android:paddingStart="4dp" />
`)
		if got := len(runRtlCompat(t, rtlCompatManifest("14", "16", ""), body)); got != 0 {
			t.Errorf("target 16: expected 0 findings, got %d", got)
		}
		unknown := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="test"><application/></manifest>`
		if got := len(runRtlCompat(t, unknown, body)); got != 0 {
			t.Errorf("no SDK levels: expected 0 findings, got %d", got)
		}
	})
	t.Run("tools:ignore RtlCompat suppresses", func(t *testing.T) {
		findings := runRtlCompat(t, pre17, layout(`    <TextView tools:ignore="RtlCompat" android:paddingStart="4dp" />
`))
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", rtlCompatLines(findings))
		}
	})
}
