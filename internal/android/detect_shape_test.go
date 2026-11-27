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
