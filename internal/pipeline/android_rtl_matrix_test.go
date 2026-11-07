package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// The oracle tables below are the findings Android Lint 32.4.1 reports for synthetic projects (one layout with
// ten Left/Right/Start/End/textAlignment views), recorded from its command line. H, S, C and E count the
// RtlHardcoded, RtlSymmetry and RtlCompat findings and name the RtlEnabled message ("target" for the
// targetSdk message, "proj" for the unset-supportsRtl one). A zero SDK level means the manifest omits it.

type rtlOracleRow struct {
	supportsRtl string
	target, min int
	hardcoded   int
	symmetry    int
	compat      int
	enabled     string
}

type rtlLibOracleRow struct {
	appSupportsRtl string
	appTarget      int
	appMin         int
	libMin         int
	hardcoded      int
	symmetry       int
	compat         int
	enabled        string
}

var rtlStandaloneAppOracle = []rtlOracleRow{
	{"false", 0, 0, 5, 0, 0, ""},
	{"false", 0, 14, 5, 0, 0, ""},
	{"false", 0, 21, 0, 0, 0, ""},
	{"false", 14, 0, 5, 0, 0, ""},
	{"false", 14, 14, 5, 0, 0, ""},
	{"false", 14, 21, 0, 0, 0, ""},
	{"false", 16, 0, 5, 0, 0, ""},
	{"false", 16, 14, 5, 0, 0, ""},
	{"false", 16, 21, 0, 0, 0, ""},
	{"false", 17, 0, 5, 0, 0, ""},
	{"false", 17, 14, 5, 0, 0, ""},
	{"false", 17, 21, 0, 0, 0, ""},
	{"false", 21, 0, 5, 0, 0, ""},
	{"false", 21, 14, 5, 0, 0, ""},
	{"false", 21, 21, 0, 0, 0, ""},
	{"true", 0, 0, 5, 0, 0, "target"},
	{"true", 0, 14, 5, 0, 0, "target"},
	{"true", 0, 21, 7, 1, 0, ""},
	{"true", 14, 0, 5, 0, 0, "target"},
	{"true", 14, 14, 5, 0, 0, "target"},
	{"true", 14, 21, 0, 0, 0, ""},
	{"true", 16, 0, 5, 0, 0, "target"},
	{"true", 16, 14, 5, 0, 0, "target"},
	{"true", 16, 21, 0, 0, 0, ""},
	{"true", 17, 0, 7, 1, 2, ""},
	{"true", 17, 14, 7, 1, 2, ""},
	{"true", 17, 21, 7, 1, 0, ""},
	{"true", 21, 0, 7, 1, 2, ""},
	{"true", 21, 14, 7, 1, 2, ""},
	{"true", 21, 21, 7, 1, 0, ""},
	{"unset", 0, 0, 5, 0, 0, ""},
	{"unset", 0, 14, 5, 0, 0, ""},
	{"unset", 0, 21, 7, 1, 0, "proj"},
	{"unset", 14, 0, 5, 0, 0, ""},
	{"unset", 14, 14, 5, 0, 0, ""},
	{"unset", 14, 21, 0, 0, 0, ""},
	{"unset", 16, 0, 5, 0, 0, ""},
	{"unset", 16, 14, 5, 0, 0, ""},
	{"unset", 16, 21, 0, 0, 0, ""},
	{"unset", 17, 0, 7, 1, 2, "proj"},
	{"unset", 17, 14, 7, 1, 2, "proj"},
	{"unset", 17, 21, 7, 1, 0, "proj"},
	{"unset", 21, 0, 7, 1, 2, "proj"},
	{"unset", 21, 14, 7, 1, 2, "proj"},
	{"unset", 21, 21, 7, 1, 0, "proj"},
}

var rtlLibraryOracle = []rtlLibOracleRow{
	{"false", 14, 21, 14, 0, 0, 0, ""},
	{"false", 14, 21, 21, 0, 0, 0, ""},
	{"false", 17, 21, 14, 0, 0, 0, ""},
	{"false", 17, 21, 21, 0, 0, 0, ""},
	{"false", 21, 21, 14, 0, 0, 0, ""},
	{"false", 21, 21, 21, 0, 0, 0, ""},
	{"true", 14, 21, 14, 0, 0, 0, ""},
	{"true", 14, 21, 21, 0, 0, 0, ""},
	{"true", 17, 21, 14, 7, 1, 0, ""},
	{"true", 17, 21, 21, 7, 1, 0, ""},
	{"true", 21, 21, 14, 7, 1, 0, ""},
	{"true", 21, 21, 21, 7, 1, 0, ""},
	{"unset", 14, 21, 14, 0, 0, 0, ""},
	{"unset", 14, 21, 21, 0, 0, 0, ""},
	{"unset", 17, 21, 14, 7, 1, 0, "proj"},
	{"unset", 17, 21, 21, 7, 1, 0, "proj"},
	{"unset", 21, 14, 14, 7, 1, 2, "proj"},
	{"unset", 21, 14, 21, 2, 1, 0, "proj"},
	{"unset", 21, 21, 14, 7, 1, 0, "proj"},
	{"unset", 21, 21, 21, 7, 1, 0, "proj"},
}

const rtlMatrixLayout = `<?xml version="1.0" encoding="utf-8"?>
<RelativeLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent" android:layout_height="match_parent">
  <TextView android:id="@+id/padl" android:layout_width="wrap_content" android:layout_height="wrap_content" android:paddingLeft="4dp"/>
  <TextView android:id="@+id/padlr" android:layout_width="wrap_content" android:layout_height="wrap_content" android:paddingLeft="4dp" android:paddingRight="8dp"/>
  <TextView android:id="@+id/marl" android:layout_width="wrap_content" android:layout_height="wrap_content" android:layout_marginLeft="4dp" android:layout_marginRight="4dp"/>
  <TextView android:id="@+id/grav" android:layout_width="wrap_content" android:layout_height="wrap_content" android:gravity="left"/>
  <TextView android:id="@+id/lgrav" android:layout_width="wrap_content" android:layout_height="wrap_content" android:layout_gravity="right"/>
  <TextView android:id="@+id/apl" android:layout_width="wrap_content" android:layout_height="wrap_content" android:layout_alignParentLeft="true"/>
  <TextView android:id="@+id/drl" android:layout_width="wrap_content" android:layout_height="wrap_content" android:drawableLeft="@android:drawable/star_on"/>
  <TextView android:id="@+id/tal" android:layout_width="wrap_content" android:layout_height="wrap_content" android:textAlignment="viewStart"/>
  <TextView android:id="@+id/pads" android:layout_width="wrap_content" android:layout_height="wrap_content" android:paddingStart="4dp" android:paddingEnd="4dp"/>
  <TextView android:id="@+id/mars" android:layout_width="wrap_content" android:layout_height="wrap_content" android:layout_marginStart="4dp"/>
</RelativeLayout>
`

const rtlEmptyLayout = `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent"/>
`

func writeRtlFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// rtlManifestBody writes a manifest; a zero level is omitted and supportsRtl "unset" omits the attribute.
func rtlManifestBody(supportsRtl string, target, min int, application bool) string {
	var b strings.Builder
	b.WriteString("<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" package=\"synth.pkg\">\n")
	if target > 0 || min > 0 {
		b.WriteString("  <uses-sdk")
		if min > 0 {
			fmt.Fprintf(&b, " android:minSdkVersion=\"%d\"", min)
		}
		if target > 0 {
			fmt.Fprintf(&b, " android:targetSdkVersion=\"%d\"", target)
		}
		b.WriteString(" />\n")
	}
	if application {
		b.WriteString("  <application android:label=\"x\"")
		if supportsRtl != "unset" {
			fmt.Fprintf(&b, " android:supportsRtl=\"%s\"", supportsRtl)
		}
		b.WriteString("/>\n")
	}
	b.WriteString("</manifest>\n")
	return b.String()
}

type rtlCounts struct {
	hardcoded, symmetry, compat int
	enabled                     []string
}

func (c rtlCounts) String() string {
	return fmt.Sprintf("H=%d S=%d C=%d E=%v", c.hardcoded, c.symmetry, c.compat, c.enabled)
}

// runRtlProject runs the four RTL rules through the Android phase over the project under root.
func runRtlProject(t *testing.T, root string) rtlCounts {
	t.Helper()
	var active []*api.Rule
	for _, id := range []string{"RtlHardcodedResource", "RtlSymmetryResource", "RtlCompatResource", "RtlEnabledManifest"} {
		active = append(active, findV2RuleForTest(t, id))
	}
	result, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
		Project:     android.DetectProject([]string{root}),
		ActiveRules: active,
		Dispatcher:  rules.NewDispatcher(active, nil),
	})
	if err != nil {
		t.Fatalf("AndroidPhase.Run: %v", err)
	}
	var c rtlCounts
	for _, f := range result.Findings.Findings() {
		switch f.Rule {
		case "RtlHardcodedResource":
			c.hardcoded++
		case "RtlSymmetryResource":
			c.symmetry++
		case "RtlCompatResource":
			c.compat++
		case "RtlEnabledManifest":
			switch {
			case strings.Contains(f.Message, "You must set"):
				c.enabled = append(c.enabled, "target")
			case strings.Contains(f.Message, "does not explicitly"):
				c.enabled = append(c.enabled, "proj")
			}
		}
	}
	sort.Strings(c.enabled)
	return c
}

func (c rtlCounts) enabledKind() string {
	if len(c.enabled) == 0 {
		return ""
	}
	return c.enabled[0]
}

// TestRtlRulesMatchLintForStandaloneApps compares krit with Lint 32.4.1 for an app module over supportsRtl,
// targetSdk and minSdk. Undeclared levels take Lint's defaults (minSdk 1, targetSdk = minSdk).
func TestRtlRulesMatchLintForStandaloneApps(t *testing.T) {
	for _, row := range rtlStandaloneAppOracle {
		name := fmt.Sprintf("rtl=%s/target=%d/min=%d", row.supportsRtl, row.target, row.min)
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeRtlFile(t, filepath.Join(root, "settings.gradle.kts"), "include(\":app\")\n")
			writeRtlFile(t, filepath.Join(root, "app", "build.gradle.kts"), "plugins { id(\"com.android.application\") }\n")
			writeRtlFile(t, filepath.Join(root, "app", "src", "main", "AndroidManifest.xml"), rtlManifestBody(row.supportsRtl, row.target, row.min, true))
			writeRtlFile(t, filepath.Join(root, "app", "src", "main", "res", "layout", "main.xml"), rtlMatrixLayout)
			got := runRtlProject(t, root)
			if got.hardcoded != row.hardcoded || got.symmetry != row.symmetry || got.compat != row.compat || got.enabledKind() != row.enabled {
				t.Fatalf("got %s, Lint reports H=%d S=%d C=%d E=%q", got, row.hardcoded, row.symmetry, row.compat, row.enabled)
			}
		})
	}
}

// TestRtlRulesMatchLintForLibraries compares krit with Lint 32.4.1 for a library module (no <application>) that an
// app includes: the app's manifest and SDK levels decide. One deliberate difference: Lint's command line shares
// the "manifest says supportsRtl" state between modules, so it never reports the library's RtlEnabled when the
// app sets supportsRtl; krit keeps the per-module state of Gradle's partial analysis and reports it.
func TestRtlRulesMatchLintForLibraries(t *testing.T) {
	for _, row := range rtlLibraryOracle {
		name := fmt.Sprintf("app rtl=%s target=%d min=%d/lib min=%d", row.appSupportsRtl, row.appTarget, row.appMin, row.libMin)
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeRtlFile(t, filepath.Join(root, "settings.gradle.kts"), "include(\":app\")\ninclude(\":lib\")\n")
			writeRtlFile(t, filepath.Join(root, "app", "build.gradle.kts"), "plugins { id(\"com.android.application\") }\ndependencies { implementation(project(\":lib\")) }\n")
			writeRtlFile(t, filepath.Join(root, "app", "src", "main", "AndroidManifest.xml"), rtlManifestBody(row.appSupportsRtl, row.appTarget, row.appMin, true))
			writeRtlFile(t, filepath.Join(root, "app", "src", "main", "res", "layout", "main.xml"), rtlEmptyLayout)
			writeRtlFile(t, filepath.Join(root, "lib", "build.gradle.kts"), "plugins { id(\"com.android.library\") }\n")
			writeRtlFile(t, filepath.Join(root, "lib", "src", "main", "AndroidManifest.xml"), rtlManifestBody("unset", 0, row.libMin, false))
			writeRtlFile(t, filepath.Join(root, "lib", "src", "main", "res", "layout", "main.xml"), rtlMatrixLayout)
			got := runRtlProject(t, root)
			wantEnabled := row.enabled
			if row.appSupportsRtl == "true" && row.appTarget >= 17 && wantEnabled == "" {
				wantEnabled = "proj"
			}
			if got.hardcoded != row.hardcoded || got.symmetry != row.symmetry || got.compat != row.compat || got.enabledKind() != wantEnabled {
				t.Fatalf("got %s, Lint reports H=%d S=%d C=%d E=%q", got, row.hardcoded, row.symmetry, row.compat, wantEnabled)
			}
		})
	}
}

// TestRtlRulesSkipLibrariesNoAppIncludes checks a library no app depends on: Lint analyzes it alone, and
// without an <application> RTL never applies.
func TestRtlRulesSkipLibrariesNoAppIncludes(t *testing.T) {
	root := t.TempDir()
	writeRtlFile(t, filepath.Join(root, "settings.gradle.kts"), "include(\":app\")\ninclude(\":orphan\")\ninclude(\":testonly\")\n")
	writeRtlFile(t, filepath.Join(root, "app", "build.gradle.kts"), "plugins { id(\"com.android.application\") }\ndependencies { testImplementation(project(\":testonly\")) }\n")
	writeRtlFile(t, filepath.Join(root, "app", "src", "main", "AndroidManifest.xml"), rtlManifestBody("unset", 21, 21, true))
	for _, lib := range []string{"orphan", "testonly"} {
		writeRtlFile(t, filepath.Join(root, lib, "build.gradle.kts"), "plugins { id(\"com.android.library\") }\n")
		writeRtlFile(t, filepath.Join(root, lib, "src", "main", "AndroidManifest.xml"), rtlManifestBody("unset", 21, 21, false))
		writeRtlFile(t, filepath.Join(root, lib, "src", "main", "res", "layout", "main.xml"), rtlMatrixLayout)
	}
	if got := runRtlProject(t, root); got.hardcoded+got.symmetry+got.compat != 0 || len(got.enabled) != 0 {
		t.Fatalf("got %s, want no findings", got)
	}
}
