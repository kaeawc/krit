package rename

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestApply_KotlinSamePackageRename(t *testing.T) {
	dir := t.TempDir()
	declPath := filepath.Join(dir, "OldName.kt")
	usePath := filepath.Join(dir, "Feature.kt")

	writeFile(t, declPath, ""+
		"package com.example\n"+
		"\n"+
		"class OldName {\n"+
		"    fun greet() = \"hi\"\n"+
		"}\n")
	writeFile(t, usePath, ""+
		"package com.example\n"+
		"\n"+
		"fun useIt(): OldName = OldName()\n")

	plan := buildKotlinPlan(t, []string{declPath, usePath}, "com.example.OldName", "com.example.NewName")
	if got := plan.CandidateCount(); got == 0 {
		t.Fatalf("expected candidates, got 0")
	}

	res, err := Apply(plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.FilesChanged != 2 {
		t.Fatalf("FilesChanged = %d, want 2", res.FilesChanged)
	}

	got := readFile(t, declPath)
	if !strings.Contains(got, "class NewName") || strings.Contains(got, "OldName") {
		t.Fatalf("decl file content unexpected:\n%s", got)
	}
	got = readFile(t, usePath)
	if strings.Contains(got, "OldName") {
		t.Fatalf("ref file still mentions OldName:\n%s", got)
	}
	if !strings.Contains(got, "NewName") {
		t.Fatalf("ref file missing NewName:\n%s", got)
	}
}

func TestApply_DryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "OldName.kt")
	writeFile(t, path, "package com.example\n\nclass OldName\n")

	plan := buildKotlinPlan(t, []string{path}, "com.example.OldName", "com.example.NewName")
	res, err := DryRunApply(plan)
	if err != nil {
		t.Fatalf("DryRunApply: %v", err)
	}
	if res.FilesChanged != 1 {
		t.Fatalf("FilesChanged = %d, want 1", res.FilesChanged)
	}
	if got := readFile(t, path); !strings.Contains(got, "OldName") {
		t.Fatalf("dry-run modified file: %s", got)
	}
}

func TestApply_KotlinCrossPackageRename(t *testing.T) {
	dir := t.TempDir()
	declPath := filepath.Join(dir, "OldName.kt")
	usePath := filepath.Join(dir, "Feature.kt")

	writeFile(t, declPath, ""+
		"package com.example.foo\n"+
		"\n"+
		"class OldName {\n"+
		"    fun greet() = \"hi\"\n"+
		"}\n")
	writeFile(t, usePath, ""+
		"package com.example.user\n"+
		"\n"+
		"import com.example.foo.OldName\n"+
		"\n"+
		"fun useIt(): OldName = OldName()\n")

	plan := buildKotlinPlan(t, []string{declPath, usePath}, "com.example.foo.OldName", "com.example.bar.NewName")
	if got := plan.CandidateCount(); got == 0 {
		t.Fatalf("expected candidates, got 0")
	}

	if _, err := Apply(plan); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := readFile(t, declPath)
	if !strings.Contains(got, "package com.example.bar") {
		t.Fatalf("decl file package not rewritten:\n%s", got)
	}
	if !strings.Contains(got, "class NewName") {
		t.Fatalf("decl file class not renamed:\n%s", got)
	}

	got = readFile(t, usePath)
	if !strings.Contains(got, "import com.example.bar.NewName") {
		t.Fatalf("import not rewritten:\n%s", got)
	}
	if strings.Contains(got, "OldName") || strings.Contains(got, "com.example.foo") {
		t.Fatalf("residual OldName/com.example.foo:\n%s", got)
	}
	if !strings.Contains(got, "NewName()") {
		t.Fatalf("call site not rewritten:\n%s", got)
	}
}

func TestApply_JavaCrossPackageRename(t *testing.T) {
	dir := t.TempDir()
	declPath := filepath.Join(dir, "OldName.java")
	usePath := filepath.Join(dir, "Feature.java")

	writeFile(t, declPath, ""+
		"package com.example.foo;\n"+
		"\n"+
		"public class OldName {\n"+
		"    public String greet() { return \"hi\"; }\n"+
		"}\n")
	writeFile(t, usePath, ""+
		"package com.example.user;\n"+
		"\n"+
		"import com.example.foo.OldName;\n"+
		"\n"+
		"public class Feature {\n"+
		"    public OldName useIt() { return new OldName(); }\n"+
		"}\n")

	files, errs := scanner.ScanJavaFiles([]string{declPath, usePath}, 1)
	for _, e := range errs {
		if e != nil {
			t.Fatalf("scan: %v", e)
		}
	}
	idx := scanner.BuildIndex(nil, 1, files...)
	target, err := ParseTarget("com.example.foo.OldName", "com.example.bar.NewName")
	if err != nil {
		t.Fatalf("ParseTarget: %v", err)
	}
	plan := BuildPlanWithFiles(idx, target, files)
	if _, err := Apply(plan); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := readFile(t, declPath)
	if !strings.Contains(got, "package com.example.bar;") {
		t.Fatalf("java decl package not rewritten:\n%s", got)
	}
	if !strings.Contains(got, "class NewName") {
		t.Fatalf("java decl class not renamed:\n%s", got)
	}

	got = readFile(t, usePath)
	if !strings.Contains(got, "import com.example.bar.NewName;") {
		t.Fatalf("java import not rewritten:\n%s", got)
	}
	if strings.Contains(got, "OldName") {
		t.Fatalf("residual OldName in java use:\n%s", got)
	}
}

func TestApply_RejectsDifferentPackage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Other.kt")
	writeFile(t, path, "package com.other\n\nclass OldName\n")

	plan := buildKotlinPlan(t, []string{path}, "com.example.OldName", "com.example.NewName")
	if got := plan.CandidateCount(); got != 0 {
		t.Fatalf("expected 0 candidates for different-package OldName, got %d", got)
	}
}

func buildKotlinPlan(t *testing.T, paths []string, fromFQN, toFQN string) Plan {
	t.Helper()
	files, errs := scanner.ScanFiles(paths, 1)
	for _, e := range errs {
		if e != nil {
			t.Fatalf("scan: %v", e)
		}
	}
	idx := scanner.BuildIndex(files, 1)
	target, err := ParseTarget(fromFQN, toFQN)
	if err != nil {
		t.Fatalf("ParseTarget: %v", err)
	}
	return BuildPlan(idx, target)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
