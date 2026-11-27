package android

import (
	"path/filepath"
	"testing"
)

func TestProjectShapeChangedBy(t *testing.T) {
	root := filepath.FromSlash("/repo")
	at := func(rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }
	project := &Project{
		ManifestPaths: []string{at("app/src/main/AndroidManifest.xml")},
		ResDirs:       []string{at("app/src/main/res")},
	}
	for _, tc := range []struct {
		path   string
		exists bool
		want   bool
	}{
		{"app/src/main/AndroidManifest.xml", true, false},
		{"app/src/main/AndroidManifest.xml", false, true},
		{"lib/src/main/AndroidManifest.xml", true, true},
		{"lib/src/main/AndroidManifest.xml", false, false},
		{"app/src/main/res/layout/main.xml", true, false},
		{"app/src/main/res/layout/main.xml", false, false},
		{"lib/src/main/res/layout/main.xml", true, true},
		{"app/src/main/assets/data.xml", true, false},
	} {
		if got := project.ShapeChangedBy(at(tc.path), tc.exists); got != tc.want {
			t.Errorf("ShapeChangedBy(%s, exists=%v) = %v, want %v", tc.path, tc.exists, got, tc.want)
		}
	}
	if !(*Project)(nil).ShapeChangedBy(at("app/src/main/AndroidManifest.xml"), true) {
		t.Error("a nil project must report a change")
	}
}

func TestProjectHasPathsUnder(t *testing.T) {
	root := filepath.FromSlash("/repo")
	at := func(rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }
	project := &Project{
		ManifestPaths: []string{at("app/src/main/AndroidManifest.xml")},
		ResDirs:       []string{at("app/src/main/res")},
		GradlePaths:   []string{at("app/build.gradle.kts")},
	}
	for rel, want := range map[string]bool{
		"app/src/main":     true,
		"app/src/main/res": true,
		"app":              true,
		"app/src/debug":    false,
		"app/src/mai":      false,
		"lib":              false,
	} {
		if got := project.HasPathsUnder(at(rel)); got != want {
			t.Errorf("HasPathsUnder(%s) = %v, want %v", rel, got, want)
		}
	}
}

func TestAffectsDetection(t *testing.T) {
	for path, want := range map[string]bool{
		"/repo/app/src/main/AndroidManifest.xml":          true,
		"/repo/app/src/debug/AndroidManifest.xml":         true,
		"/repo/app/src/main/res/drawable/ic.png":          true,
		"/repo/app/src/main/res/font/a.ttf":               true,
		"/repo/app/src/main/res/layout/main.xml":          true,
		"/repo/app/src/main/java/com/example/A.kt":        false,
		"/repo/app/src/main/assets/data.xml":              false,
		"/repo/app/build.gradle.kts":                      false,
		"/repo/app/src/main/java/com/example/res/ic.png":  false,
		"/repo/app/src/main/resources/AndroidManifest.md": false,
	} {
		if got := AffectsDetection(filepath.FromSlash(path)); got != want {
			t.Errorf("AffectsDetection(%s) = %v, want %v", path, got, want)
		}
	}
}

func TestDetectionSkipsDir(t *testing.T) {
	for name, want := range map[string]bool{
		".cxx": true, ".gradle": true, "build": true, "node_modules": true, "__pycache__": true,
		"app": false, "src": false, "builds": false, "res": false,
	} {
		if got := DetectionSkipsDir(name); got != want {
			t.Errorf("DetectionSkipsDir(%q) = %v, want %v", name, got, want)
		}
	}
}
