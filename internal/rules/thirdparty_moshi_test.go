package rules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// moshiRules returns the registered MoshiUsage rules.
func moshiRules(t *testing.T) []*api.Rule {
	t.Helper()
	var out []*api.Rule
	for _, r := range api.Registry {
		if r != nil && r.Category == moshiRuleSet && strings.HasPrefix(r.ID, "MoshiUsage") {
			out = append(out, r)
		}
	}
	if len(out) != len(moshiIssues) {
		t.Fatalf("registered %d MoshiUsage rules, want %d", len(out), len(moshiIssues))
	}
	return out
}

// runMoshiRules runs every MoshiUsage rule over files the way the cross-file
// phase does and returns "path:line:col: rule: message" lines, with paths
// relative to root and suppressed findings dropped.
func runMoshiRules(t *testing.T, root string, files []*scanner.File) []string {
	t.Helper()
	var out []string
	for _, r := range moshiRules(t) {
		collector := scanner.NewFindingCollector(0)
		r.Check(&api.Context{Rule: r, ParsedFiles: files, Collector: collector})
		for _, f := range collector.Columns().Findings() {
			var file *scanner.File
			for _, candidate := range files {
				if candidate.Path == f.File {
					file = candidate
				}
			}
			if file != nil && file.Suppression.IsSuppressedAtColumn(f.Rule, f.RuleSet, f.Line, f.Col) {
				continue
			}
			rel, err := filepath.Rel(root, f.File)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, fmt.Sprintf("%s:%d:%d: %s: %s", filepath.ToSlash(rel), f.Line, f.Col, f.Rule, f.Message))
		}
	}
	sort.Strings(out)
	return out
}

func parseMoshiFixtureFile(t *testing.T, path string) *scanner.File {
	t.Helper()
	var file *scanner.File
	var err error
	if strings.HasSuffix(path, ".java") {
		file, err = scanner.ParseJavaFile(context.Background(), path)
	} else {
		file, err = scanner.ParseFile(context.Background(), path)
	}
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	file.Suppression = scanner.BuildSuppressionFilter(file, nil, nil, "").WithRuleAliases(AllSuppressionAliases())
	return file
}

// TestMoshiUsageLintParity runs the MoshiUsage rules over each project fixture
// under tests/fixtures/android-lint/MoshiUsage*/ and compares the findings with
// expected.txt. expected.txt is the output of Android Lint 9.4.0-rc01 running
// slack-lint-checks 0.11.1 on the same sources (plus Moshi, moshix, Gson and
// androidx.annotation stubs and kotlin-stdlib), keeping only the IDs krit
// ports. A directory's krit-divergence.txt lists the deliberate differences:
// "+ <finding>" lines krit reports beyond Lint and "- <finding>" lines it
// does not, each explained by the comment lines above it.
func TestMoshiUsageLintParity(t *testing.T) {
	registered := map[string]bool{}
	for _, r := range moshiRules(t) {
		registered[r.ID] = true
	}
	dirs, err := filepath.Glob(filepath.Join(repoRoot(t), "tests", "fixtures", "android-lint", "MoshiUsage*", "*"))
	if err != nil {
		t.Fatal(err)
	}
	ran := 0
	for _, dir := range dirs {
		expectedPath := filepath.Join(dir, "expected.txt")
		data, err := os.ReadFile(expectedPath)
		if err != nil {
			t.Fatalf("%s: every MoshiUsage fixture directory needs an expected.txt: %v", dir, err)
		}
		ran++
		name, _ := filepath.Rel(filepath.Join(repoRoot(t), "tests", "fixtures", "android-lint"), dir)
		t.Run(filepath.ToSlash(name), func(t *testing.T) {
			var want []string
			for _, line := range strings.Split(string(data), "\n") {
				if line == "" {
					continue
				}
				fields := strings.SplitN(line, ": ", 3)
				if len(fields) == 3 && registered[fields[1]] {
					want = append(want, line)
				}
			}
			want = applyMoshiDivergences(t, filepath.Join(dir, "krit-divergence.txt"), want)
			sort.Strings(want)
			var files []*scanner.File
			if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && (strings.HasSuffix(path, ".kt") || strings.HasSuffix(path, ".java")) {
					files = append(files, parseMoshiFixtureFile(t, path))
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			got := runMoshiRules(t, dir, files)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("findings differ from Lint\n got:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
			}
		})
	}
	if ran == 0 {
		t.Fatal("no MoshiUsage fixture directories found")
	}
}

// applyMoshiDivergences applies a fixture's documented differences from Lint.
func applyMoshiDivergences(t *testing.T, path string, want []string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return want
	}
	if err != nil {
		t.Fatal(err)
	}
	explained := false
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "#"):
			explained = true
		case strings.HasPrefix(line, "+ "):
			if !explained {
				t.Fatalf("%s: divergence %q has no explaining comment", path, line)
			}
			want = append(want, strings.TrimPrefix(line, "+ "))
		case strings.HasPrefix(line, "- "):
			if !explained {
				t.Fatalf("%s: divergence %q has no explaining comment", path, line)
			}
			drop := strings.TrimPrefix(line, "- ")
			kept := want[:0]
			found := false
			for _, w := range want {
				if w == drop && !found {
					found = true
					continue
				}
				kept = append(kept, w)
			}
			if !found {
				t.Fatalf("%s: divergence %q removes a finding Lint does not report", path, line)
			}
			want = kept
		case strings.TrimSpace(line) == "":
		default:
			t.Fatalf("%s: unrecognized line %q", path, line)
		}
	}
	return want
}

// moshiFindingsForSources runs the MoshiUsage rules over in-memory sources.
func moshiFindingsForSources(t *testing.T, sources map[string]string) []string {
	t.Helper()
	root := t.TempDir()
	var files []*scanner.File
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(sources[name]), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, parseMoshiFixtureFile(t, path))
	}
	return runMoshiRules(t, root, files)
}

// The pass abstains where Lint would evaluate a value krit cannot prove from
// source. These are known false negatives, not findings.
func TestMoshiUsageEnumAbstainsOnUnprovenValues(t *testing.T) {
	got := moshiFindingsForSources(t, map[string]string{
		"Enums.kt": `package com.example

import com.squareup.moshi.JsonClass

const val GENERATE = true
const val GENERATOR = ""

// Lint casts generateAdapter to a literal and crashes on a reference (ClassCastException,
// aborting the file); krit abstains.
@JsonClass(generateAdapter = GENERATE)
enum class ConstantFlag { ONE }

// Known false negative: Lint evaluates GENERATOR to "" and reports MoshiUsageEnumMissingUnknown;
// krit cannot tell a blank generator from a custom one, so it abstains.
@JsonClass(generateAdapter = false, generator = GENERATOR)
enum class ConstantGenerator { ONE }

// An escape in the generator literal is not decoded, so the generator is unknown.
// Lint reads "custom" here and skips the enum, so krit staying silent matches.
@JsonClass(generateAdapter = true, generator = "\u0063ustom")
enum class EscapedGenerator { ONE }
`,
	})
	if len(got) != 0 {
		t.Fatalf("expected no findings, got:\n  %s", strings.Join(got, "\n  "))
	}
}

// Moshi annotations count only when their FQN is proven: imported, written in
// full, reached through a wildcard import, or declared in the same package.
func TestMoshiUsageEnumResolvesAnnotationFQN(t *testing.T) {
	got := moshiFindingsForSources(t, map[string]string{
		"Wildcard.kt": `package com.example

import com.squareup.moshi.*

@JsonClass(generateAdapter = false)
enum class Wildcard { ONE }
`,
		"Aliased.kt": `package com.example

import com.squareup.moshi.JsonClass as MoshiClass

@MoshiClass(generateAdapter = false)
enum class Aliased { ONE }
`,
		"SamePackage.kt": `package com.squareup.moshi

@JsonClass(generateAdapter = false)
enum class SamePackage { ONE }
`,
		"Shadowed.kt": `package com.example.other

import com.squareup.moshi.*

annotation class JsonClass(val generateAdapter: Boolean)

// The file's own JsonClass shadows the wildcard import.
@JsonClass(generateAdapter = false)
enum class Shadowed { ONE }
`,
		"OtherImport.kt": `package com.example.another

import com.example.custom.JsonClass
import com.squareup.moshi.JsonAdapter

@JsonClass(generateAdapter = false)
enum class OtherImport { ONE }
`,
	})
	want := []string{
		"Aliased.kt:6:12: MoshiUsageEnumMissingUnknown: Enums serialized with Moshi must reserve the first member as UNKNOWN.",
		"SamePackage.kt:4:12: MoshiUsageEnumMissingUnknown: Enums serialized with Moshi must reserve the first member as UNKNOWN.",
		"Wildcard.kt:6:12: MoshiUsageEnumMissingUnknown: Enums serialized with Moshi must reserve the first member as UNKNOWN.",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestMoshiScreamingSnakeCase(t *testing.T) {
	for in, want := range map[string]string{
		"UNKNOWN":       "UNKNOWN",
		"lower":         "LOWER",
		"camelCase":     "CAMEL_CASE",
		"TWO_WORDS":     "TWO_WORDS",
		"V2":            "V2",
		"_leading":      "LEADING",
		"kebab-case":    "KEBAB_CASE",
		"dotted.name":   "DOTTED_NAME",
		"HTTPResponse":  "HTTPRESPONSE",
		"snake__double": "SNAKE_DOUBLE",
	} {
		if got := moshiScreamingSnakeCase(in); got != want {
			t.Errorf("moshiScreamingSnakeCase(%q) = %q, want %q", in, got, want)
		}
	}
}

// A lint.xml that ignores a MoshiUsage ID turns the krit rule off, because the
// rules declare api.Rule.LintIssue. A same-named ID on a rule without the flag
// outside android-lint does not count as a Lint issue.
func TestMoshiUsageHonorsLintXMLIgnore(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "lint.xml"), []byte(`<lint>
  <issue id="MoshiUsageEnumCasing" severity="ignore"/>
</lint>`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadAndMergeDefaults("", root)
	if err != nil {
		t.Fatal(err)
	}
	if !androidLintTurnsOff(cfg, registeredRule(t, "MoshiUsageEnumCasing")) {
		t.Error("lint.xml ignore did not turn off MoshiUsageEnumCasing")
	}
	if androidLintTurnsOff(cfg, registeredRule(t, "MoshiUsageEnumMissingUnknown")) {
		t.Error("lint.xml ignore of another ID turned off MoshiUsageEnumMissingUnknown")
	}
	if androidLintIgnoresRule(cfg.AndroidLint(), &api.Rule{ID: "MoshiUsageEnumCasingCopy", Category: "style"}) {
		t.Error("a rule without LintIssue outside android-lint was treated as a Lint issue")
	}
}

// Known false negatives: Lint evaluates `const val` arguments of @JsonClass on
// classes and reports GenerateAdapterShouldBeTrue on ConstFlag, and
// BlankGenerator plus UseData on ConstGenerator. krit does not fold constants,
// so it abstains on the whole class.
func TestMoshiUsageClassAbstainsOnConstantArguments(t *testing.T) {
	got := moshiFindingsForSources(t, map[string]string{
		"Classes.kt": `package com.example

import com.squareup.moshi.JsonClass

const val NO = false
const val GEN = ""

@JsonClass(generateAdapter = NO)
data class ConstFlag(val a: Int)

@JsonClass(generateAdapter = true, generator = GEN)
class ConstGenerator(val a: Int)
`,
	})
	if len(got) != 0 {
		t.Fatalf("expected no findings, got:\n  %s", strings.Join(got, "\n  "))
	}
}

// A subtype of a sealed type whose @JsonClass generator is not a literal
// abstains: krit cannot tell whether the parent is moshi-sealed. Lint 9.4.0
// with slack-lints 0.11.1 reports nothing on this input either.
func TestMoshiUsageSealedSubtypeAbstainsOnConstantParentGenerator(t *testing.T) {
	got := moshiFindingsForSources(t, map[string]string{
		"Animal.kt": `package com.example

import com.squareup.moshi.JsonClass
import dev.zacsweers.moshix.sealed.annotations.TypeLabel

const val GENERATOR = "sealed:type"

@JsonClass(generateAdapter = true, generator = GENERATOR)
sealed class Animal {
  @TypeLabel("dog")
  @JsonClass(generateAdapter = true)
  data class Dog(val name: String) : Animal()
}
`,
		"Cat.kt": `package com.example

data class Cat(val name: String) : Animal()
`,
	})
	if len(got) != 0 {
		t.Fatalf("expected no findings, got:\n  %s", strings.Join(got, "\n  "))
	}
}

// The supertype resolver follows Kotlin scoping: a nested class shadows a
// top-level class with the same name, so a subtype of the nested non-sealed
// Base is not a moshi-sealed subtype.
func TestMoshiUsageSealedSupertypeScoping(t *testing.T) {
	got := moshiFindingsForSources(t, map[string]string{
		"Base.kt": `package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true, generator = "sealed:type")
sealed class Base

class Outer {
  open class Base

  class Child : Base()
}
`,
	})
	if len(got) != 0 {
		t.Fatalf("expected no findings, got:\n  %s", strings.Join(got, "\n  "))
	}
}

// A `null` default is an anonymous token in the tree; it still counts as a
// default value (moshi's GeneratedAdaptersTest has this shape).
func TestMoshiUsageNullDefaultCountsAsInitialized(t *testing.T) {
	got := moshiFindingsForSources(t, map[string]string{
		"Lambdas.kt": `package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class LambdaTypeNames(
  val id: String,
  @Transient val simple: ((String) -> Boolean)? = null,
)

@JsonClass(generateAdapter = true)
class NullDefault(val id: String, plain: String? = null)
`,
	})
	want := []string{"Lambdas.kt:12:7: MoshiUsageUseData: Model classes should be immutable data classes."}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestMoshiSnakeToCamel(t *testing.T) {
	for in, want := range map[string]string{
		"userId":      "userId",
		"user_id":     "userId",
		"kebab-case":  "kebabCase",
		"_leading":    "leading",
		"trailing_":   "trailing",
		"two__scores": "twoScores",
	} {
		if got := moshiSnakeToCamel(in); got != want {
			t.Errorf("moshiSnakeToCamel(%q) = %q, want %q", in, got, want)
		}
	}
}

// An @AdaptedBy target whose supertype chain leaves the project cannot be
// proven a non-adapter (the library base may extend JsonAdapter), so the
// pass abstains. With the library on its classpath Lint resolves the chain.
func TestMoshiUsageAdaptedByAbstainsOnLibrarySupertype(t *testing.T) {
	got := moshiFindingsForSources(t, map[string]string{
		"Adapted.kt": `package com.example

import com.example.library.LibraryAdapterBase
import dev.zacsweers.moshix.adapters.AdaptedBy

@AdaptedBy(FromLibrary::class)
data class Adapted(val a: Int)

class FromLibrary : LibraryAdapterBase<Adapted>()
`,
	})
	if len(got) != 0 {
		t.Fatalf("expected no findings, got:\n  %s", strings.Join(got, "\n  "))
	}
}
