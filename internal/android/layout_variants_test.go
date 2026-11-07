package android

import (
	"os"
	"path/filepath"
	"testing"
)

func writeVariantLayouts(t *testing.T) string {
	t.Helper()
	res := t.TempDir()
	for _, dir := range []string{"layout", "layout-land", "layout-sw600dp"} {
		if err := os.MkdirAll(filepath.Join(res, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		xml := `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent"><TextView android:id="@+id/t_` + dir[len("layout"):] + `" android:layout_width="wrap_content" android:layout_height="wrap_content"/></LinearLayout>`
		if err := os.WriteFile(filepath.Join(res, dir, "main.xml"), []byte(xml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return res
}

func TestAllLayoutsKeepsQualifierVariants(t *testing.T) {
	idx, err := ScanResourceDir(writeVariantLayouts(t))
	if err != nil {
		t.Fatal(err)
	}
	all := idx.AllLayouts()
	if len(all) != 3 {
		t.Fatalf("AllLayouts() = %d layouts, want 3 variants", len(all))
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].FilePath >= all[i].FilePath {
			t.Fatalf("AllLayouts() not ordered by path: %q then %q", all[i-1].FilePath, all[i].FilePath)
		}
	}
	if got := idx.Layouts["main"]; got == nil || filepath.Base(filepath.Dir(got.FilePath)) != "layout" {
		t.Fatalf("Layouts[main] should be the unqualified variant, got %+v", got)
	}
}

func TestMergeKeepsDefaultLayoutAndAllVariants(t *testing.T) {
	a, b := newResourceIndex(), newResourceIndex()
	def := &Layout{Name: "main", FilePath: "res/layout/main.xml"}
	land := &Layout{Name: "main", FilePath: "res/layout-land/main.xml"}
	a.Layouts["main"], a.LayoutVariants = def, []*Layout{def}
	b.Layouts["main"], b.LayoutVariants = land, []*Layout{land}
	a.mergeFrom(b)
	if a.Layouts["main"] != def {
		t.Fatal("qualifier variant replaced the default layout")
	}
	if len(a.AllLayouts()) != 2 {
		t.Fatalf("AllLayouts() = %d, want 2", len(a.AllLayouts()))
	}
}

func TestResourceCacheRoundTripKeepsLayoutVariants(t *testing.T) {
	idx, err := ScanResourceDir(writeVariantLayouts(t))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(cloneResourceIndex(idx).AllLayouts()); got != 3 {
		t.Fatalf("clone kept %d variants, want 3", got)
	}
}
