package android

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSiblingSourceSetResDirs(t *testing.T) {
	src := filepath.Join(t.TempDir(), "app", "src")
	for _, set := range []string{"main", "debug", "testing", "test", "androidTest", "testDebug"} {
		if err := os.MkdirAll(filepath.Join(src, set, "res"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := SiblingSourceSetResDirs(filepath.Join(src, "main", "res"))
	want := []string{filepath.Join(src, "debug", "res"), filepath.Join(src, "testing", "res")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("siblings = %v, want %v", got, want)
	}
	if got := SiblingSourceSetResDirs(filepath.Join(t.TempDir(), "res")); got != nil {
		t.Fatalf("non source-set res dir siblings = %v, want nil", got)
	}
}
