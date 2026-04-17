package android

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanFileReferences_FindsKotlinAndXML(t *testing.T) {
	tmp := t.TempDir()

	// Kotlin file with a direct file reference.
	ktFile := filepath.Join(tmp, "App.kt")
	os.WriteFile(ktFile, []byte(`package com.example
val icon = "icon.png"
val other = "logo.svg"
`), 0644)

	// XML file with a direct file reference.
	xmlFile := filepath.Join(tmp, "layout.xml")
	os.WriteFile(xmlFile, []byte(`<ImageView android:src="icon.png" />
<ImageView android:src="@drawable/icon" />
`), 0644)

	// Java file with direct reference.
	javaFile := filepath.Join(tmp, "Helper.java")
	os.WriteFile(javaFile, []byte(`String name = "icon.png";
`), 0644)

	refs := ScanFileReferences([]string{tmp}, "icon.png")

	if len(refs) != 3 {
		t.Fatalf("expected 3 references, got %d: %+v", len(refs), refs)
	}

	// Verify paths.
	paths := map[string]bool{}
	for _, r := range refs {
		paths[r.Path] = true
	}
	for _, expected := range []string{ktFile, xmlFile, javaFile} {
		if !paths[expected] {
			t.Errorf("expected reference in %s, not found", expected)
		}
	}
}

func TestScanFileReferences_DoesNotFindResourceName(t *testing.T) {
	tmp := t.TempDir()

	// XML file that only uses the resource name (no extension) -- this is safe.
	xmlFile := filepath.Join(tmp, "layout.xml")
	os.WriteFile(xmlFile, []byte(`<ImageView android:src="@drawable/icon" />
`), 0644)

	refs := ScanFileReferences([]string{tmp}, "icon.png")

	if len(refs) != 0 {
		t.Errorf("expected 0 references for resource-name-only usage, got %d: %+v", len(refs), refs)
	}
}

func TestScanFileReferences_IgnoresNonSourceFiles(t *testing.T) {
	tmp := t.TempDir()

	// A PNG file itself should not be scanned.
	pngFile := filepath.Join(tmp, "icon.png")
	os.WriteFile(pngFile, []byte("icon.png"), 0644)

	// A text file should not be scanned.
	txtFile := filepath.Join(tmp, "notes.txt")
	os.WriteFile(txtFile, []byte("see icon.png for details"), 0644)

	refs := ScanFileReferences([]string{tmp}, "icon.png")

	if len(refs) != 0 {
		t.Errorf("expected 0 references from non-source files, got %d: %+v", len(refs), refs)
	}
}

func TestScanFileReferences_EmptyFileName(t *testing.T) {
	refs := ScanFileReferences([]string{"/tmp"}, "")
	if refs != nil {
		t.Errorf("expected nil for empty file name, got %v", refs)
	}
}

func TestScanFileReferences_MultipleSearchDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	os.WriteFile(filepath.Join(dir1, "A.kt"), []byte(`val x = "logo.png"`), 0644)
	os.WriteFile(filepath.Join(dir2, "B.java"), []byte(`String y = "logo.png";`), 0644)

	refs := ScanFileReferences([]string{dir1, dir2}, "logo.png")

	if len(refs) != 2 {
		t.Fatalf("expected 2 references across dirs, got %d: %+v", len(refs), refs)
	}
}

func TestScanFileReferences_LineNumbers(t *testing.T) {
	tmp := t.TempDir()

	ktFile := filepath.Join(tmp, "Test.kt")
	os.WriteFile(ktFile, []byte("line one\nline two\nval f = \"bg.png\"\nline four\n"), 0644)

	refs := ScanFileReferences([]string{tmp}, "bg.png")

	if len(refs) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(refs))
	}
	if refs[0].Line != 3 {
		t.Errorf("expected line 3, got %d", refs[0].Line)
	}
	if refs[0].Text != `val f = "bg.png"` {
		t.Errorf("unexpected text: %q", refs[0].Text)
	}
}

func TestScanFileReferences_SubdirectoryTraversal(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "src", "main", "kotlin")
	os.MkdirAll(sub, 0755)

	ktFile := filepath.Join(sub, "Deep.kt")
	os.WriteFile(ktFile, []byte(`val img = "banner.png"`), 0644)

	refs := ScanFileReferences([]string{tmp}, "banner.png")

	if len(refs) != 1 {
		t.Fatalf("expected 1 reference in subdirectory, got %d", len(refs))
	}
	if refs[0].Path != ktFile {
		t.Errorf("expected path %s, got %s", ktFile, refs[0].Path)
	}
}
