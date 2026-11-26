package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/manifest"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// workManifestSourceFindings writes files under a temp project and runs one manifest-source rule on it.
func workManifestSourceFindings(t *testing.T, ruleID string, files map[string]string) []scanner.Finding {
	t.Helper()
	dir := t.TempDir()
	var sources []*scanner.File
	var manifests []*manifest.Manifest
	for rel, body := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for rel := range files {
		path := filepath.Join(dir, rel)
		switch filepath.Ext(rel) {
		case ".kt":
			f, err := scanner.ParseFile(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			sources = append(sources, f)
		case ".xml":
			parsed, err := android.ParseManifest(path)
			if err != nil {
				t.Fatal(err)
			}
			manifests = append(manifests, workConvertManifest(android.ConvertManifest(parsed, path)))
		}
	}
	for _, r := range api.Registry {
		if r.ID == ruleID {
			cols, _ := NewDispatcher([]*api.Rule{r}, nil).RunManifestSource(manifests, sources)
			out := cols.Findings()
			for i := range out {
				out[i].File, _ = filepath.Rel(dir, out[i].File)
			}
			return out
		}
	}
	t.Fatalf("rule %s not registered", ruleID)
	return nil
}

// workConvertManifest keeps the fields these rules read; the pipeline's converter lives in a package that imports
// this one.
func workConvertManifest(m *android.ConvertedManifest) *manifest.Manifest {
	return &manifest.Manifest{Path: m.Path, Root: workConvertNode(m.Root)}
}

func workConvertNode(n *android.XMLNode) *manifest.Node {
	if n == nil {
		return nil
	}
	out := &manifest.Node{Tag: n.Tag, Line: n.Line, Col: n.Col}
	for _, a := range n.Attrs {
		out.Attrs = append(out.Attrs, manifest.Attr{Name: a.Name, Value: a.Value, Line: a.Line, Col: a.Col})
	}
	for _, c := range n.Children {
		out.Children = append(out.Children, workConvertNode(c))
	}
	return out
}

const workProviderApp = `package com.example

import android.app.Application
import androidx.work.Configuration

class App : Application(), Configuration.Provider {
    override val workManagerConfiguration: Configuration
        get() = Configuration.Builder().build()
}
`

const workPlainManifest = `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application android:name=".App" />
</manifest>
`

func TestRemoveWorkManagerInitializerHonorsToolsIgnoreAndSuppressLint(t *testing.T) {
	if got := workManifestSourceFindings(t, "RemoveWorkManagerInitializer", map[string]string{
		"app/src/main/java/App.kt":         workProviderApp,
		"app/src/main/AndroidManifest.xml": workPlainManifest,
	}); len(got) != 1 || got[0].Line != 2 || got[0].Col != 5 {
		t.Fatalf("want one finding at <application> 2:5, got %+v", got)
	}
	ignored := `<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:tools="http://schemas.android.com/tools">
    <application android:name=".App" tools:ignore="RemoveWorkManagerInitializer" />
</manifest>
`
	if got := workManifestSourceFindings(t, "RemoveWorkManagerInitializer", map[string]string{
		"app/src/main/java/App.kt":         workProviderApp,
		"app/src/main/AndroidManifest.xml": ignored,
	}); len(got) != 0 {
		t.Fatalf("tools:ignore on <application> must silence the manifest finding, got %+v", got)
	}
	suppressed := `package com.example

import android.annotation.SuppressLint
import android.app.Application
import androidx.work.Configuration

@SuppressLint("RemoveWorkManagerInitializer")
class App : Application(), Configuration.Provider {
    override val workManagerConfiguration: Configuration
        get() = Configuration.Builder().build()
}
`
	if got := workManifestSourceFindings(t, "RemoveWorkManagerInitializer", map[string]string{
		"app/src/main/java/App.kt":         suppressed,
		"app/src/main/AndroidManifest.xml": workPlainManifest,
	}); len(got) != 0 {
		t.Fatalf("Lint honors @SuppressLint on the Application class, got %+v", got)
	}
}

func TestRemoveWorkManagerInitializerLocatesInTheApplicationsModule(t *testing.T) {
	libManifest := `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application>
        <provider android:name="androidx.startup.InitializationProvider" />
    </application>
</manifest>
`
	got := workManifestSourceFindings(t, "RemoveWorkManagerInitializer", map[string]string{
		"app/build.gradle.kts":              "",
		"app/src/main/java/App.kt":          workProviderApp,
		"app/src/main/AndroidManifest.xml":  workPlainManifest,
		"zlib/build.gradle.kts":             "",
		"zlib/src/main/AndroidManifest.xml": libManifest,
	})
	if len(got) != 1 || got[0].File != filepath.Join("app", "src", "main", "AndroidManifest.xml") {
		t.Fatalf("want one finding in the app module's manifest, got %+v", got)
	}
	removedInLib := `<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:tools="http://schemas.android.com/tools">
    <application>
        <provider android:name="androidx.startup.InitializationProvider" tools:node="remove" />
    </application>
</manifest>
`
	if got := workManifestSourceFindings(t, "RemoveWorkManagerInitializer", map[string]string{
		"app/build.gradle.kts":              "",
		"app/src/main/java/App.kt":          workProviderApp,
		"app/src/main/AndroidManifest.xml":  workPlainManifest,
		"zlib/build.gradle.kts":             "",
		"zlib/src/main/AndroidManifest.xml": removedInLib,
	}); len(got) != 0 {
		t.Fatalf("a removal in any manifest of the scan counts, got %+v", got)
	}
}

func TestSpecifyForegroundServiceTypeHonorsSuppressLint(t *testing.T) {
	src := `package com.example

import android.annotation.SuppressLint
import android.app.Notification
import androidx.work.ForegroundInfo

fun reported(n: Notification) = ForegroundInfo(1, n, 8)

@SuppressLint("SpecifyForegroundServiceType")
fun suppressed(n: Notification) = ForegroundInfo(2, n, 8)
`
	got := workManifestSourceFindings(t, "SpecifyForegroundServiceType", map[string]string{
		"app/src/main/java/Fg.kt":          src,
		"app/src/main/AndroidManifest.xml": workPlainManifest,
	})
	if len(got) != 1 || got[0].Line != 7 || got[0].Message != "Missing location foregroundServiceType in the AndroidManifest.xml" {
		t.Fatalf("want only the unsuppressed location finding on line 7, got %+v", got)
	}
}
