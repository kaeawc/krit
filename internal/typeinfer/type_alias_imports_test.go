package typeinfer

import (
	"fmt"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestResolveTypeAliasImportPrecedence(t *testing.T) {
	for _, test := range []struct {
		name, source string
		extra        []string
		want         string
	}{
		{"samePackage", "package app\nimport kotlinx.coroutines.*\ntypealias Names = List<String>\nfun f(names: Names) = names as MutableList<String>", nil, "kotlin.collections.List"},
		{"samePackageBeforeWildcard", "package app\nimport other.*\ntypealias Names = List<String>\nval names: Names = listOf()", []string{"package other\nclass Names"}, "kotlin.collections.List"},
		{"wildcardAlias", "package app\nimport other.*\nval names: Names = listOf()", []string{"package other\ntypealias Names = List<String>"}, "kotlin.collections.List"},
		{"laterWildcardAlias", "package app\nimport missing.*\nimport other.*\nval names: Names = listOf()", []string{"package other\ntypealias Names = List<String>"}, "kotlin.collections.List"},
		{"duplicateWildcard", "package app\nimport other.*\nimport other.*\nval names: Names = listOf()", []string{"package other\ntypealias Names = List<String>"}, "kotlin.collections.List"},
		{"ambiguousAliases", "package app\nimport other.*\nimport another.*\nval names: Names = listOf()", []string{"package other\ntypealias Names = List<String>", "package another\ntypealias Names = Set<String>"}, ""},
		{"wildcardClassShadowsAlias", "package app\nimport other.*\nimport another.*\nval names: Names = listOf()", []string{"package other\ntypealias Names = List<String>", "package another\nclass Names"}, ""},
		{"unimportedAlias", "package app\nval names: Names = listOf()", []string{"package other\ntypealias Names = List<String>"}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := parseTestFile(t, test.source)
			file.Path = "/project/Use.kt"
			files := []*scanner.File{file}
			for index, source := range test.extra {
				extra := parseTestFile(t, source)
				extra.Path = fmt.Sprintf("/project/Extra%d.kt", index)
				files = append(files, extra)
			}
			resolver := NewResolver()
			resolver.IndexFilesParallel(files, 1)
			got := resolver.resolveTypeAlias("Names", file, resolver.imports[file.Path])
			if test.want == "" {
				if got != nil {
					t.Fatalf("want no alias expansion, got %v", got)
				}
				return
			}
			if got == nil || got.FQN != test.want {
				t.Fatalf("want %s, got %v", test.want, got)
			}
			if test.name == "samePackage" {
				parameter := flatFirstOfTypeWithText(file, "user_type", "Names")
				resolved := resolver.ResolveFlatNode(parameter, file)
				if resolved == nil || resolved.FQN != test.want {
					t.Fatalf("DontDowncast parameter: want %s, got %v", test.want, resolved)
				}
			}
		})
	}
}
