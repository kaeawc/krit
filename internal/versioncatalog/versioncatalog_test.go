package versioncatalog

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseLibraries(t *testing.T) {
	tests := []struct {
		name, src, alias, module, resolved string
		line                               int
	}{
		{"baseline", "[versions]\nk = \"1.0\"\n[libraries]\ncore = { module = \"a:b\", version.ref = \"k\" }", "core", "a:b", "1.0", 4},
		{"sub-table", "[versions]\nk = \"1.0\"\n[libraries.core]\nmodule = \"a:b\"\nversion.ref = \"k\"", "core", "a:b", "1.0", 3},
		{"inline version", "[versions]\nk = \"1.0\"\n[libraries]\ncore = { module = \"a:b\", version = { ref = \"k\" } }", "core", "a:b", "1.0", 4},
		{"rich prefer", "[versions]\nk = { strictly = \"[1.0,2.0)\", prefer = \"1.5\" }\n[libraries]\ncore = { module = \"a:b\", version.ref = \"k\" }", "core", "a:b", "1.5", 4},
		{"rich strictly", "[versions]\nk = { strictly = \"1.4\" }\n[libraries]\ncore = { module = \"a:b\", version.ref = \"k\" }", "core", "a:b", "1.4", 4},
		{"literal strings", "['versions']\n'k' = '1.0'\n['libraries']\n'core' = { module = 'a:b', version.ref = 'k' }", "core", "a:b", "1.0", 4},
		{"quoted key", "[versions]\nk = \"1.0\"\n[libraries]\n\"core\" = { module = \"a:b\", version.ref = \"k\" }", "core", "a:b", "1.0", 4},
		{"spaced header", "[ versions ]\nk = \"1.0\"\n[ libraries ]\ncore = { module = \"a:b\", version.ref = \"k\" }", "core", "a:b", "1.0", 4},
		{"top-level dotted", "versions.k = \"1.0\"\nlibraries.core = { module = \"a:b\", version.ref = \"k\" }", "core", "a:b", "1.0", 2},
		{"string with version", "[libraries]\ncore = \"a:b:1.0\"", "core", "a:b", "1.0", 2},
		{"string without version", "[libraries]\ncore = \"a:b\"", "core", "a:b", "", 2},
		{"group and name", "[libraries]\ncore = { group = \"a\", name = \"b\", version = \"1.0\" }", "core", "a:b", "1.0", 2},
		{"unknown ref", "[libraries]\ncore = { module = \"a:b\", version.ref = \"missing\" }", "core", "a:b", "", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, err := Parse([]byte(tt.src))
			if err != nil {
				t.Fatal(err)
			}
			if len(cat.Libraries) != 1 {
				t.Fatalf("libraries: %+v", cat.Libraries)
			}
			got := cat.Libraries[0]
			if got.Alias != tt.alias || got.Pos.Line != tt.line || got.Module() != tt.module || got.Resolved != tt.resolved {
				t.Fatalf("library: %+v, module %q", got, got.Module())
			}
		})
	}
}

func TestParseVersionsAndPlugins(t *testing.T) {
	src := "[versions]\nk = \"1.0\\\"#x\"\nrich = { require = \"1.0\", strictly = \"1.4\", prefer = \"1.5\", reject = [\"1.2\"], rejectAll = true }\n[plugins]\nplain = \"com.x:1.0\"\nbare = \"com.y\"\nref = { id = \"com.z\", version.ref = \"rich\" }\n"
	cat, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Versions) != 2 || cat.Versions[0].Alias != "k" || cat.Versions[0].Pos.Line != 2 || cat.Versions[0].Version.Require != "1.0\"#x" || !cat.Versions[0].Version.Plain {
		t.Fatalf("versions: %+v", cat.Versions)
	}
	rich := cat.Versions[1].Version
	if rich.Effective() != "1.5" || !reflect.DeepEqual(rich.Reject, []string{"1.2"}) || !rich.RejectAll {
		t.Fatalf("rich version: %+v", rich)
	}
	if len(cat.Plugins) != 3 {
		t.Fatalf("plugins: %+v", cat.Plugins)
	}
	for i, want := range []struct {
		alias, id, resolved string
		line                int
	}{
		{"plain", "com.x", "1.0", 5},
		{"bare", "com.y", "", 6},
		{"ref", "com.z", "1.5", 7},
	} {
		got := cat.Plugins[i]
		if got.Alias != want.alias || got.Pos.Line != want.line || got.ID != want.id || got.Resolved != want.resolved {
			t.Fatalf("plugin %d: %+v", i, got)
		}
	}
	if !cat.Plugins[0].Version.Plain || cat.Plugins[2].Version.Ref != "rich" {
		t.Fatalf("plugin versions: %+v", cat.Plugins)
	}
}

func TestParseBundlesAndOrder(t *testing.T) {
	src := "[libraries]\ne = \"g:e\"\nc = \"g:c\"\na = \"g:a\"\nd = \"g:d\"\nb = \"g:b\"\n[bundles]\nfirst = [\"e\", \"c\"]\nsecond = [\n  \"a\", # comment\n  \"b\",\n]\n"
	cat, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for i, alias := range []string{"e", "c", "a", "d", "b"} {
		got := cat.Libraries[i]
		if got.Alias != alias || got.Pos.Line != i+2 || got.Module() != "g:"+alias || got.Resolved != "" {
			t.Fatalf("library %d: %+v", i, got)
		}
	}
	if len(cat.Bundles) != 2 || cat.Bundles[0].Alias != "first" || cat.Bundles[0].Pos.Line != 8 || !reflect.DeepEqual(cat.Bundles[0].Libraries, []string{"e", "c"}) || cat.Bundles[1].Alias != "second" || cat.Bundles[1].Pos.Line != 9 || !reflect.DeepEqual(cat.Bundles[1].Libraries, []string{"a", "b"}) {
		t.Fatalf("bundles: %+v", cat.Bundles)
	}
}

func TestParseSkipsUnrecognizedShapes(t *testing.T) {
	cat, err := Parse([]byte("[libraries]\nbad = { version = \"1.0\" }\n[metadata]\nname = \"ignored\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Libraries) != 0 || len(cat.Versions) != 0 || len(cat.Plugins) != 0 || len(cat.Bundles) != 0 {
		t.Fatalf("unexpected entries: %+v", cat)
	}
}

func TestParseArrayOfTablesLibraries(t *testing.T) {
	cat, err := Parse([]byte("[[libraries]]\nmodule = \"a:b\"\nversion = \"1.0\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Libraries) != 0 {
		t.Fatalf("libraries: %+v, want none", cat.Libraries)
	}
}

func TestParseSubTableVersionReference(t *testing.T) {
	src := "[versions.k]\nstrictly = \"1.4\"\n[libraries]\ncore = { module = \"a:b\", version.ref = \"k\" }\n"
	cat, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Libraries) != 1 || cat.Libraries[0].Resolved != "1.4" {
		t.Fatalf("libraries: %+v, want resolved version 1.4", cat.Libraries)
	}
	if len(cat.Versions) != 1 || cat.Versions[0].Alias != "k" || cat.Versions[0].Pos.Line != 1 {
		t.Fatalf("versions: %+v, want alias k at line 1", cat.Versions)
	}
}

func TestPositionsUseAliasKeysAndTableHeaders(t *testing.T) {
	cat, err := Parse([]byte("versions.k = \"1.0\"\n  libraries.core = \"a:b\"\n  [ libraries.other ]\nmodule = \"c:d\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Versions) != 1 || cat.Versions[0].Pos != (Position{Line: 1, Column: 10}) {
		t.Fatalf("version position: %+v", cat.Versions)
	}
	if len(cat.Libraries) != 2 || cat.Libraries[0].Pos != (Position{Line: 2, Column: 13}) || cat.Libraries[1].Pos != (Position{Line: 3, Column: 3}) {
		t.Fatalf("library positions: %+v", cat.Libraries)
	}
	positions, err := entryPositions([]byte("libraries.core = { version = { ref = \"k\" } }"))
	if err != nil {
		t.Fatal(err)
	}
	if got := positions[path("libraries", "core", "version", "ref")]; got != (Position{Line: 1, Column: 32}) {
		t.Fatalf("inline key position: %+v", got)
	}
}

func TestParseInvalidTOML(t *testing.T) {
	for _, src := range []string{
		"[versions]\nk = \"1\"\nk = \"2\"",
		"[versions]\nk = \"unfinished",
		"[versions\nk = \"1\"",
	} {
		cat, err := Parse([]byte(src))
		if err == nil || cat != nil {
			t.Fatalf("Parse(%q) = %+v, %v", src, cat, err)
		}
	}
}

func TestParseFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "catalog-*.toml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("[libraries]\ncore = \"a:b:1.0\"\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	cat, err := ParseFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Libraries) != 1 || cat.Libraries[0].Alias != "core" || cat.Libraries[0].Pos.Line != 2 || cat.Libraries[0].Module() != "a:b" || cat.Libraries[0].Resolved != "1.0" {
		t.Fatalf("catalog: %+v", cat)
	}
	if cat, err := ParseFile(filepath.Join(t.TempDir(), "missing.toml")); err == nil || cat != nil {
		t.Fatalf("missing file: %+v, %v", cat, err)
	}
}
