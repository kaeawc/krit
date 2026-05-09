package rename

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestParseTarget(t *testing.T) {
	target, err := ParseTarget("com.example.OldName", "com.example.NewName")
	if err != nil {
		t.Fatalf("ParseTarget returned error: %v", err)
	}
	if target.FromName != "OldName" {
		t.Fatalf("FromName = %q, want OldName", target.FromName)
	}
	if target.ToName != "NewName" {
		t.Fatalf("ToName = %q, want NewName", target.ToName)
	}
}

func TestBuildPlan(t *testing.T) {
	target, err := ParseTarget("com.example.OldName", "com.example.NewName")
	if err != nil {
		t.Fatalf("ParseTarget returned error: %v", err)
	}

	idx := scanner.BuildIndexFromData(
		[]scanner.Symbol{
			{Name: "OldName", Kind: "class", File: "src/main/kotlin/com/example/OldName.kt", Line: 3},
			{Name: "OtherName", Kind: "class", File: "src/main/kotlin/com/example/OtherName.kt", Line: 5},
		},
		[]scanner.Reference{
			{Name: "OldName", File: "src/main/kotlin/com/example/OldName.kt", Line: 3},
			{Name: "OldName", File: "src/main/kotlin/com/example/Feature.kt", Line: 10},
			{Name: "OldName", File: "src/main/java/com/example/FeatureJava.java", Line: 7},
			{Name: "OtherName", File: "src/main/kotlin/com/example/Feature.kt", Line: 14},
		},
	)

	plan := BuildPlan(idx, target)
	summary := plan.Summary()

	if summary.Declarations != 1 {
		t.Fatalf("Declarations = %d, want 1", summary.Declarations)
	}
	if summary.References != 3 {
		t.Fatalf("References = %d, want 3", summary.References)
	}
	if summary.Files != 3 {
		t.Fatalf("Files = %d, want 3", summary.Files)
	}
	if plan.CandidateCount() != 4 {
		t.Fatalf("CandidateCount = %d, want 4", plan.CandidateCount())
	}
	if got, want := plan.Files[0], "src/main/java/com/example/FeatureJava.java"; got != want {
		t.Fatalf("Files[0] = %q, want %q", got, want)
	}
}

func TestFileContext_MatchesFQN(t *testing.T) {
	imp := func(fqn string) importInfo { return importInfo{FQN: fqn} }
	cases := []struct {
		name string
		ctx  fileContext
		ref  string
		fqn  string
		want bool
	}{
		{
			name: "explicit import matches",
			ctx:  fileContext{Imports: map[string]importInfo{"OldName": imp("com.example.OldName")}},
			ref:  "OldName",
			fqn:  "com.example.OldName",
			want: true,
		},
		{
			name: "explicit import to different fqn rejects",
			ctx:  fileContext{Imports: map[string]importInfo{"OldName": imp("com.other.OldName")}},
			ref:  "OldName",
			fqn:  "com.example.OldName",
			want: false,
		},
		{
			name: "same package matches without import",
			ctx:  fileContext{Package: "com.example"},
			ref:  "OldName",
			fqn:  "com.example.OldName",
			want: true,
		},
		{
			name: "same package but explicit import overrides",
			ctx: fileContext{
				Package: "com.example",
				Imports: map[string]importInfo{"OldName": imp("com.other.OldName")},
			},
			ref:  "OldName",
			fqn:  "com.example.OldName",
			want: false,
		},
		{
			name: "wildcard import matches",
			ctx:  fileContext{Wildcards: map[string]importInfo{"com.example": {}}},
			ref:  "OldName",
			fqn:  "com.example.OldName",
			want: true,
		},
		{
			name: "alias resolves to fqn",
			ctx:  fileContext{Aliases: map[string]importInfo{"Alias": imp("com.example.OldName")}},
			ref:  "Alias",
			fqn:  "com.example.OldName",
			want: true,
		},
		{
			name: "unrelated file rejects",
			ctx:  fileContext{Package: "com.other"},
			ref:  "OldName",
			fqn:  "com.example.OldName",
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.ctx.matchesFQN(tc.ref, tc.fqn)
			if got != tc.want {
				t.Fatalf("matchesFQN = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseImports(t *testing.T) {
	t.Run("kotlin", func(t *testing.T) {
		assertEq := func(got parsedImport, want parsedImport, name string) {
			if got != want {
				t.Errorf("%s: got %+v want %+v", name, got, want)
			}
		}
		assertEq(parseKotlinImport("com.example.Foo"), parsedImport{fqn: "com.example.Foo"}, "explicit")
		assertEq(parseKotlinImport("com.example.Bar as B"), parsedImport{fqn: "com.example.Bar", alias: "B"}, "alias")
		assertEq(parseKotlinImport("com.example.util.*"), parsedImport{wildcard: true, pkg: "com.example.util"}, "wildcard")
	})
	t.Run("java", func(t *testing.T) {
		assertEq := func(got parsedImport, want parsedImport, name string) {
			if got != want {
				t.Errorf("%s: got %+v want %+v", name, got, want)
			}
		}
		assertEq(parseJavaImport("com.example.Foo"), parsedImport{fqn: "com.example.Foo"}, "explicit")
		assertEq(parseJavaImport("static com.example.Bar.baz"), parsedImport{fqn: "com.example.Bar.baz"}, "static")
		assertEq(parseJavaImport("com.example.util.*"), parsedImport{wildcard: true, pkg: "com.example.util"}, "wildcard")
	})
}

func TestBuildPlan_FiltersByFQN(t *testing.T) {
	target, err := ParseTarget("com.example.OldName", "com.example.NewName")
	if err != nil {
		t.Fatalf("ParseTarget: %v", err)
	}

	idx := scanner.BuildIndexFromData(
		[]scanner.Symbol{
			{Name: "OldName", FQN: "com.example.OldName", Kind: "class", File: "a.kt", Line: 3},
			{Name: "OldName", FQN: "com.other.OldName", Kind: "class", File: "b.kt", Line: 3},
		},
		[]scanner.Reference{
			{Name: "OldName", File: "a.kt", Line: 5},
			{Name: "OldName", File: "b.kt", Line: 5},
		},
	)

	plan := BuildPlan(idx, target)
	if len(plan.Declarations) != 1 {
		t.Fatalf("Declarations = %d, want 1", len(plan.Declarations))
	}
	if plan.Declarations[0].FQN != "com.example.OldName" {
		t.Fatalf("Declarations[0].FQN = %q", plan.Declarations[0].FQN)
	}
}
