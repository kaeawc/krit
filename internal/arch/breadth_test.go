package arch

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func makeFile(path string, lines []string) *scanner.File {
	return &scanner.File{
		Path:  path,
		Lines: lines,
	}
}

func TestImportBreadth_NoImports(t *testing.T) {
	f := makeFile("Foo.kt", []string{
		"package com.example",
		"",
		"class Foo",
	})
	if got := ImportBreadth(f); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestImportBreadth_FewImports(t *testing.T) {
	f := makeFile("Foo.kt", []string{
		"package com.example",
		"",
		"import com.example.util.StringUtils",
		"import com.example.util.NumberUtils",
		"import com.example.model.User",
	})
	// 2 distinct packages: com.example.util and com.example.model
	if got := ImportBreadth(f); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}

func TestImportBreadth_ManyPackages(t *testing.T) {
	lines := []string{"package com.test"}
	for i := 0; i < 15; i++ {
		lines = append(lines, "import com.pkg"+string(rune('a'+i))+".Foo")
	}
	f := makeFile("Big.kt", lines)
	if got := ImportBreadth(f); got != 15 {
		t.Errorf("expected 15, got %d", got)
	}
}

func TestImportBreadth_WildcardImports(t *testing.T) {
	f := makeFile("Wild.kt", []string{
		"package com.example",
		"import com.example.util.*",
	})
	if got := ImportBreadth(f); got != 1 {
		t.Errorf("expected 1, got %d", got)
	}
}

func TestImportBreadth_SamePackageDedup(t *testing.T) {
	f := makeFile("Dedup.kt", []string{
		"package com.example",
		"import com.example.util.A",
		"import com.example.util.B",
		"import com.example.util.C",
	})
	if got := ImportBreadth(f); got != 1 {
		t.Errorf("expected 1 (same package deduped), got %d", got)
	}
}

func TestImportBreadth_AliasImports(t *testing.T) {
	f := makeFile("Alias.kt", []string{
		"package com.example",
		"import com.example.util.StringUtils as SU",
		"import com.example.model.User as U",
	})
	if got := ImportBreadth(f); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}

func TestFindBroadFiles_Threshold(t *testing.T) {
	narrow := makeFile("narrow.kt", []string{
		"package com.example",
		"import com.a.Foo",
		"import com.b.Bar",
	})
	broad := makeFile("broad.kt", []string{
		"package com.example",
		"import com.a.A",
		"import com.b.B",
		"import com.c.C",
		"import com.d.D",
		"import com.e.E",
		"import com.f.F",
		"import com.g.G",
		"import com.h.H",
		"import com.i.I",
		"import com.j.J",
		"import com.k.K",
	})

	files := []*scanner.File{narrow, broad}
	results := FindBroadFiles(files, 10)

	if len(results) != 1 {
		t.Fatalf("expected 1 broad file, got %d", len(results))
	}
	if results[0].Path != "broad.kt" {
		t.Errorf("expected broad.kt, got %s", results[0].Path)
	}
	if results[0].PackageCount != 11 {
		t.Errorf("expected 11 packages, got %d", results[0].PackageCount)
	}
}
