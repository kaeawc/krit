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

	newDeclPath := filepath.Join(dir, "NewName.kt")
	got := readFile(t, newDeclPath)
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

	got := readFile(t, filepath.Join(dir, "NewName.kt"))
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

	got := readFile(t, filepath.Join(dir, "NewName.java"))
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

func TestApply_RenamesMatchingFile(t *testing.T) {
	dir := t.TempDir()
	declPath := filepath.Join(dir, "OldName.kt")
	writeFile(t, declPath, "package com.example\n\nclass OldName\n")

	plan := buildKotlinPlan(t, []string{declPath}, "com.example.OldName", "com.example.NewName")
	res, err := Apply(plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(res.Moves) != 1 {
		t.Fatalf("Moves = %d, want 1", len(res.Moves))
	}
	want := filepath.Join(dir, "NewName.kt")
	if res.Moves[0].To != want {
		t.Fatalf("Moves[0].To = %q, want %q", res.Moves[0].To, want)
	}
	if _, err := os.Stat(declPath); !os.IsNotExist(err) {
		t.Fatalf("old file still exists: err=%v", err)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("new file missing: %v", err)
	}
}

func TestApply_DoesNotRenameMismatchedFile(t *testing.T) {
	dir := t.TempDir()
	declPath := filepath.Join(dir, "MyFile.kt")
	writeFile(t, declPath, "package com.example\n\nclass OldName\n")

	plan := buildKotlinPlan(t, []string{declPath}, "com.example.OldName", "com.example.NewName")
	res, err := Apply(plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(res.Moves) != 0 {
		t.Fatalf("Moves = %d, want 0", len(res.Moves))
	}
	if _, err := os.Stat(declPath); err != nil {
		t.Fatalf("file should still exist: %v", err)
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

func TestApply_IgnoresStringLiterals(t *testing.T) {
	dir := t.TempDir()
	declPath := filepath.Join(dir, "OldName.kt")
	usePath := filepath.Join(dir, "Feature.kt")
	writeFile(t, declPath, "package com.example\n\nclass OldName\n")
	writeFile(t, usePath, ""+
		"package com.example\n"+
		"\n"+
		"fun greeting(): String = \"OldName is here\"\n")

	plan := buildKotlinPlan(t, []string{declPath, usePath}, "com.example.OldName", "com.example.NewName")
	if _, err := Apply(plan); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := readFile(t, usePath)
	if !strings.Contains(got, "\"OldName is here\"") {
		t.Fatalf("string literal got rewritten: %s", got)
	}
}

func TestApply_PreservesPackageTrailingComments(t *testing.T) {
	dir := t.TempDir()
	declPath := filepath.Join(dir, "OldName.kt")
	writeFile(t, declPath, ""+
		"package com.example\n"+
		"\n"+
		"// keep this comment\n"+
		"class OldName\n")

	plan := buildKotlinPlan(t, []string{declPath}, "com.example.OldName", "com.bar.NewName")
	if _, err := Apply(plan); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := readFile(t, filepath.Join(dir, "NewName.kt"))
	if !strings.Contains(got, "// keep this comment") {
		t.Fatalf("post-package comment was dropped:\n%s", got)
	}
	if !strings.Contains(got, "package com.bar") {
		t.Fatalf("package not rewritten:\n%s", got)
	}
}

func TestApply_PreservesKotlinAliasOnCrossPackageRename(t *testing.T) {
	dir := t.TempDir()
	declPath := filepath.Join(dir, "OldName.kt")
	usePath := filepath.Join(dir, "Feature.kt")

	writeFile(t, declPath, "package com.example.foo\n\nclass OldName\n")
	writeFile(t, usePath, ""+
		"package com.example.user\n"+
		"\n"+
		"import com.example.foo.OldName as Aliased\n"+
		"\n"+
		"fun useIt(): Aliased = Aliased()\n")

	plan := buildKotlinPlan(t, []string{declPath, usePath}, "com.example.foo.OldName", "com.example.bar.NewName")
	if _, err := Apply(plan); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := readFile(t, usePath)
	if !strings.Contains(got, "import com.example.bar.NewName as Aliased") {
		t.Fatalf("alias not preserved on import rewrite:\n%s", got)
	}
	if !strings.Contains(got, "fun useIt(): Aliased = Aliased()") {
		t.Fatalf("alias usage was rewritten:\n%s", got)
	}
}

func TestApply_InsertsImportForOrphanedSamePackageReference(t *testing.T) {
	dir := t.TempDir()
	declPath := filepath.Join(dir, "OldName.kt")
	usePath := filepath.Join(dir, "Feature.kt")

	writeFile(t, declPath, "package com.example.foo\n\nclass OldName\n")
	writeFile(t, usePath, ""+
		"package com.example.foo\n"+
		"\n"+
		"fun useIt(): OldName = OldName()\n")

	plan := buildKotlinPlan(t, []string{declPath, usePath}, "com.example.foo.OldName", "com.example.bar.NewName")
	if _, err := Apply(plan); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := readFile(t, usePath)
	if !strings.Contains(got, "import com.example.bar.NewName") {
		t.Fatalf("expected inserted import, got:\n%s", got)
	}
	if !strings.Contains(got, "fun useIt(): NewName = NewName()") {
		t.Fatalf("call sites not rewritten:\n%s", got)
	}
}

func TestApply_MovesDeclarationFileToNewPackageDir(t *testing.T) {
	dir := t.TempDir()
	srcRoot := filepath.Join(dir, "src", "main", "kotlin")
	oldDir := filepath.Join(srcRoot, "com", "example", "foo")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	declPath := filepath.Join(oldDir, "OldName.kt")
	writeFile(t, declPath, "package com.example.foo\n\nclass OldName\n")

	plan := buildKotlinPlan(t, []string{declPath}, "com.example.foo.OldName", "com.example.bar.NewName")
	res, err := Apply(plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(res.Moves) != 1 {
		t.Fatalf("Moves = %d, want 1", len(res.Moves))
	}
	wantDest := filepath.Join(srcRoot, "com", "example", "bar", "NewName.kt")
	if res.Moves[0].To != wantDest {
		t.Fatalf("Moves[0].To = %q, want %q", res.Moves[0].To, wantDest)
	}
	if _, err := os.Stat(declPath); !os.IsNotExist(err) {
		t.Fatalf("old file still exists: %v", err)
	}
	if _, err := os.Stat(wantDest); err != nil {
		t.Fatalf("new file missing: %v", err)
	}
}

func TestApply_KeepsFileInPlaceWhenLayoutDoesNotMirrorPackage(t *testing.T) {
	// Path has no com/example/foo segments — flat layout. Even though
	// the rename crosses packages, the directory stays where it is.
	dir := t.TempDir()
	declPath := filepath.Join(dir, "OldName.kt")
	writeFile(t, declPath, "package com.example.foo\n\nclass OldName\n")

	plan := buildKotlinPlan(t, []string{declPath}, "com.example.foo.OldName", "com.example.bar.NewName")
	res, err := Apply(plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(res.Moves) != 1 {
		t.Fatalf("Moves = %d, want 1", len(res.Moves))
	}
	wantDest := filepath.Join(dir, "NewName.kt")
	if res.Moves[0].To != wantDest {
		t.Fatalf("Moves[0].To = %q, want %q", res.Moves[0].To, wantDest)
	}
}

func TestRemapPackageDir(t *testing.T) {
	cases := []struct {
		dir, oldPkg, newPkg string
		want                string
		ok                  bool
	}{
		{"src/main/kotlin/com/foo", "com.foo", "com.bar", "src/main/kotlin/com/bar", true},
		{"src/main/kotlin/com/foo/sub", "com.foo", "com.bar", "src/main/kotlin/com/foo/sub", false},
		{"com/foo", "com.foo", "com.bar", "com/bar", true},
		{"flat", "com.foo", "com.bar", "flat", false},
	}
	for _, tc := range cases {
		got, ok := remapPackageDir(tc.dir, tc.oldPkg, tc.newPkg)
		if ok != tc.ok || got != tc.want {
			t.Errorf("remapPackageDir(%q,%q,%q) = (%q,%v), want (%q,%v)", tc.dir, tc.oldPkg, tc.newPkg, got, ok, tc.want, tc.ok)
		}
	}
}

func TestApply_RejectsIdenticalFromAndTo(t *testing.T) {
	target, err := ParseTarget("com.example.OldName", "com.example.OldName")
	if err == nil {
		t.Fatalf("expected ParseTarget to reject identical FQNs, got target=%+v", target)
	}
}

func TestValidatePlan_RejectsAmbiguousDeclarations(t *testing.T) {
	target, _ := ParseTarget("com.example.OldName", "com.example.NewName")
	plan := Plan{
		Target: target,
		Declarations: []scanner.Symbol{
			{Name: "OldName", FQN: "com.example.OldName", File: "a.kt"},
			{Name: "OldName", FQN: "com.example.OldName", File: "b.kt"},
		},
	}
	if err := ValidatePlan(plan); err == nil {
		t.Fatalf("expected ValidatePlan to reject duplicate declarations")
	}
}

func TestValidatePlan_RejectsOccupiedDestination(t *testing.T) {
	target, _ := ParseTarget("com.example.OldName", "com.example.NewName")
	plan := Plan{
		Target: target,
		Declarations: []scanner.Symbol{
			{Name: "OldName", FQN: "com.example.OldName", File: "a.kt", Line: 3},
		},
		Conflicts: []scanner.Symbol{
			{Name: "NewName", FQN: "com.example.NewName", File: "b.kt", Line: 7},
		},
	}
	err := ValidatePlan(plan)
	if err == nil {
		t.Fatalf("expected ValidatePlan to reject rename into occupied FQN")
	}
	if !strings.Contains(err.Error(), "b.kt:7") || !strings.Contains(err.Error(), "com.example.NewName") {
		t.Errorf("error %q should mention conflicting site b.kt:7 and dest FQN", err.Error())
	}
}

func TestBuildPlan_PopulatesConflictsWhenDestinationOccupied(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "A.kt")
	b := filepath.Join(dir, "B.kt")
	writeFile(t, a, "package com.example\n\nclass OldName\n")
	writeFile(t, b, "package com.example\n\nclass NewName\n")

	plan := buildKotlinPlan(t, []string{a, b}, "com.example.OldName", "com.example.NewName")
	if len(plan.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict at destination FQN; got %d (%+v)", len(plan.Conflicts), plan.Conflicts)
	}
	if plan.Conflicts[0].FQN != "com.example.NewName" {
		t.Errorf("conflict FQN = %q; want com.example.NewName", plan.Conflicts[0].FQN)
	}
	if err := ValidatePlan(plan); err == nil {
		t.Fatalf("ValidatePlan should reject when destination is already declared")
	}
}

func TestBuildPlan_NoConflictsWhenDestinationFree(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "A.kt")
	writeFile(t, a, "package com.example\n\nclass OldName\n")

	plan := buildKotlinPlan(t, []string{a}, "com.example.OldName", "com.example.NewName")
	if len(plan.Conflicts) != 0 {
		t.Fatalf("expected no conflicts; got %d (%+v)", len(plan.Conflicts), plan.Conflicts)
	}
	if err := ValidatePlan(plan); err != nil {
		t.Fatalf("ValidatePlan should accept rename to free FQN: %v", err)
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
