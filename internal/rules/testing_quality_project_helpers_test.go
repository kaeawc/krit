package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// runTestWithoutAssertionOnProject writes files (relative path -> source) to a temp project, indexes them
// together and runs TestWithoutAssertion over the index.
func runTestWithoutAssertionOnProject(t *testing.T, sources map[string]string) []scanner.Finding {
	t.Helper()
	root := t.TempDir()
	var files []*scanner.File
	for rel, src := range sources {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	for _, r := range api.Registry {
		if r.ID == "TestWithoutAssertion" {
			return runCrossFileRuleOnFiles(r, files...)
		}
	}
	t.Fatal("TestWithoutAssertion not registered")
	return nil
}

const recordingFakeChecking = `package fakes

class AuditLog {
    private val entries = mutableSetOf<String>()
    fun record(name: String) { entries.add(name) }
    fun consume(name: String) {
        check(entries.remove(name)) { "not entries: $name" }
    }
}
`

const recordingFakeNonChecking = `package fakes

class AuditLog {
    private val entries = mutableSetOf<String>()
    fun record(name: String) { entries.add(name) }
    fun consume(name: String): Boolean = entries.remove(name)
}
`

const auditTestSource = `package app

import fakes.AuditLog
import org.junit.Test

class AuditTest {
    private val auditLog = AuditLog()

    @Test
    fun recordsEntry() {
        auditLog.record("order-1")
        auditLog.consume("order-1")
    }
}
`

func TestTestWithoutAssertion_CrossFileHelperThatChecksCountsAsAssertion(t *testing.T) {
	findings := runTestWithoutAssertionOnProject(t, map[string]string{
		"lib/src/test/kotlin/fakes/AuditLog.kt": recordingFakeChecking,
		"app/src/test/kotlin/app/AuditTest.kt":  auditTestSource,
	})
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestTestWithoutAssertion_CrossFileHelperWithoutCheckIsReported(t *testing.T) {
	findings := runTestWithoutAssertionOnProject(t, map[string]string{
		"lib/src/test/kotlin/fakes/AuditLog.kt": recordingFakeNonChecking,
		"app/src/test/kotlin/app/AuditTest.kt":  auditTestSource,
	})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestTestWithoutAssertion_CrossFileHelperChainWithinDepth(t *testing.T) {
	findings := runTestWithoutAssertionOnProject(t, map[string]string{
		"lib/src/test/kotlin/fakes/Chain.kt": `package fakes

class Chain {
    fun hopOne() { hopTwo() }
    fun hopTwo() { hopThree() }
    fun hopThree() { leaf(true) }
    fun leaf(ready: Boolean) { if (!ready) error("boom") }
}
`,
		"app/src/test/kotlin/app/ChainTest.kt": `package app

import fakes.Chain
import org.junit.Test

class ChainTest {
    @Test
    fun usesChain() {
        Chain().hopOne()
    }
}
`,
	})
	if len(findings) != 0 {
		t.Fatalf("expected no findings for a helper chain within depth, got %d: %v", len(findings), findings)
	}
}

func TestTestWithoutAssertion_CrossFileHelperBeyondDepthIsReported(t *testing.T) {
	findings := runTestWithoutAssertionOnProject(t, map[string]string{
		"lib/src/test/kotlin/fakes/Chain.kt": `package fakes

class Chain {
    fun hopOne() { hopTwo() }
    fun hopTwo() { hopThree() }
    fun hopThree() { hopFour() }
    fun hopFour() { leaf(true) }
    fun leaf(ready: Boolean) { if (!ready) error("boom") }
}
`,
		"app/src/test/kotlin/app/ChainTest.kt": `package app

import fakes.Chain
import org.junit.Test

class ChainTest {
    @Test
    fun usesChain() {
        Chain().hopOne()
    }
}
`,
	})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding beyond the recursion bound, got %d: %v", len(findings), findings)
	}
}

func TestTestWithoutAssertion_ProductionHelperDoesNotCount(t *testing.T) {
	findings := runTestWithoutAssertionOnProject(t, map[string]string{
		"lib/src/main/kotlin/fakes/AuditLog.kt": recordingFakeChecking,
		"app/src/test/kotlin/app/AuditTest.kt":  auditTestSource,
	})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding when the helper is production code, got %d: %v", len(findings), findings)
	}
}

// The cross-file result depends on the helper file, so it must be part of the index fingerprint that
// keys the cross-file findings cache.
func TestTestWithoutAssertion_CrossFileResultTracksHelperFileContent(t *testing.T) {
	fingerprint := func(helper string) string {
		root := t.TempDir()
		var files []*scanner.File
		for rel, src := range map[string]string{
			"lib/src/test/kotlin/fakes/AuditLog.kt": helper,
			"app/src/test/kotlin/app/AuditTest.kt":  auditTestSource,
		} {
			path := filepath.Join(root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(src), 0644); err != nil {
				t.Fatal(err)
			}
			file, err := scanner.ParseFile(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			files = append(files, file)
		}
		idx, _ := scanner.BuildIndexCached(t.TempDir(), files, 1, nil)
		return idx.Fingerprint
	}
	if fingerprint(recordingFakeChecking) == fingerprint(recordingFakeNonChecking) {
		t.Fatal("index fingerprint must change when a helper body changes")
	}
}

func crossFileFindings(t *testing.T, helperPath, helper, testBody string) int {
	t.Helper()
	return len(runTestWithoutAssertionOnProject(t, map[string]string{
		helperPath: helper,
		"app/src/test/kotlin/app/SomeTest.kt": `package app

import org.junit.Test

class SomeTest {
` + testBody + `
}
`,
	}))
}

const helperPathTest = "lib/src/test/kotlin/fakes/Helpers.kt"

func TestTestWithoutAssertion_HelperInTestClassDoesNotCount(t *testing.T) {
	helper := `package fakes

import org.junit.Test

class BaseTest {
    @Test fun ok() { assertTrue(true) }
    fun settleDown() { check(ready) }
}
`
	body := `    @Test
    fun usesIt() {
        settleDown()
    }`
	if n := crossFileFindings(t, helperPathTest, helper, body); n != 1 {
		t.Fatalf("expected 1 finding, got %d", n)
	}
}

func TestTestWithoutAssertion_LifecycleAnnotatedHelperDoesNotCount(t *testing.T) {
	for _, annotation := range []string{"Before", "After", "BeforeEach", "AfterEach", "BeforeClass", "AfterClass", "Rule"} {
		helper := "package fakes\n\nclass Env {\n    @" + annotation + "\n    fun prepareWorld() { check(ready) }\n}\n"
		body := "    @Test\n    fun usesIt() {\n        prepareWorld()\n    }"
		if n := crossFileFindings(t, helperPathTest, helper, body); n != 1 {
			t.Fatalf("@%s: expected 1 finding, got %d", annotation, n)
		}
	}
}

func TestTestWithoutAssertion_OverrideAndOperatorHelpersDoNotCount(t *testing.T) {
	for _, member := range []string{"override fun settleDown() { check(ready) }", "operator fun settleDown() { check(ready) }"} {
		helper := "package fakes\n\nclass Env : Base() {\n    " + member + "\n}\n"
		body := "    @Test\n    fun usesIt() {\n        settleDown()\n    }"
		if n := crossFileFindings(t, helperPathTest, helper, body); n != 1 {
			t.Fatalf("%s: expected 1 finding, got %d", member, n)
		}
	}
}

func TestTestWithoutAssertion_UnconditionalThrowStubDoesNotCount(t *testing.T) {
	for _, stub := range []string{
		`fun settleDown(): Nothing = throw NotImplementedError()`,
		`fun settleDown() { throw UnsupportedOperationException("no") }`,
		`fun settleDown() { TODO() }`,
		`fun settleDown() { error("not supported") }`,
	} {
		helper := "package fakes\n\nclass Env {\n    " + stub + "\n}\n"
		body := "    @Test\n    fun usesIt() {\n        settleDown()\n    }"
		if n := crossFileFindings(t, helperPathTest, helper, body); n != 1 {
			t.Fatalf("%s: expected 1 finding, got %d", stub, n)
		}
	}
}

func TestTestWithoutAssertion_ConditionalThrowHelperCounts(t *testing.T) {
	for _, stub := range []string{
		`fun settleDown(ok: Boolean) { if (!ok) throw IllegalStateException("bad") }`,
		`fun settleDown(ok: Boolean) { if (!ok) error("bad") }`,
		`fun settleDown(v: Int?) { v ?: error("missing") }`,
		`fun settleDown(ok: Boolean) { require(ok) }`,
	} {
		helper := "package fakes\n\nclass Env {\n    " + stub + "\n}\n"
		body := "    @Test\n    fun usesIt() {\n        settleDown(true)\n    }"
		if n := crossFileFindings(t, helperPathTest, helper, body); n != 0 {
			t.Fatalf("%s: expected 0 findings, got %d", stub, n)
		}
	}
}

func TestTestWithoutAssertion_AmbiguousNameDoesNotCount(t *testing.T) {
	findings := runTestWithoutAssertionOnProject(t, map[string]string{
		helperPathTest: `package fakes

class Checking { fun settleDown() { check(ready) } }
`,
		"lib/src/main/kotlin/prod/Prod.kt": `package prod

class Prod { fun settleDown() { println("done") } }
`,
		"app/src/test/kotlin/app/SomeTest.kt": `package app

import org.junit.Test

class SomeTest {
    @Test
    fun usesIt() {
        settleDown()
    }
}
`,
	})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for an ambiguous name, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_AmbiguousNameAcrossSupportFilesDoesNotCount(t *testing.T) {
	findings := runTestWithoutAssertionOnProject(t, map[string]string{
		helperPathTest: `package fakes

class Checking { fun settleDown() { check(ready) } }
`,
		"lib/src/test/kotlin/fakes/Other.kt": `package fakes

class Plain { fun settleDown() { counter++ } }
`,
		"app/src/test/kotlin/app/SomeTest.kt": `package app

import org.junit.Test

class SomeTest {
    @Test
    fun usesIt() {
        settleDown()
    }
}
`,
	})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_GenericNamesNeverMatch(t *testing.T) {
	for _, name := range []string{"setUp", "tearDown", "reset", "attach", "detach", "onCreate", "build", "invoke", "first", "test", "toString", "enqueue", "advanceUntilIdle", "onItemTapped", "onSomethingHappened", "getForegroundInfo", "updateCatalog"} {
		helper := "package fakes\n\nclass Env {\n    fun " + name + "() { check(ready) }\n}\n"
		body := "    @Test\n    fun usesIt() {\n        " + name + "()\n    }"
		if n := crossFileFindings(t, helperPathTest, helper, body); n != 1 {
			t.Fatalf("%s: expected 1 finding, got %d", name, n)
		}
	}
}

func TestTestWithoutAssertion_ReceiverTypeMustMatchHelperOwner(t *testing.T) {
	helper := `package fakes

class AuditLog {
    fun settleDown() { check(ready) }
}
`
	mismatch := `    private val other: UnrelatedThing = UnrelatedThing()

    @Test
    fun usesIt() {
        other.settleDown()
    }`
	if n := crossFileFindings(t, helperPathTest, helper, mismatch); n != 1 {
		t.Fatalf("mismatched receiver type: expected 1 finding, got %d", n)
	}
	match := `    private val recorder: AuditLog = AuditLog()

    @Test
    fun usesIt() {
        recorder.settleDown()
    }`
	if n := crossFileFindings(t, helperPathTest, helper, match); n != 0 {
		t.Fatalf("matching receiver type: expected 0 findings, got %d", n)
	}
}

func TestTestWithoutAssertion_ExtensionHelperMatchesAnyReceiver(t *testing.T) {
	helper := `package fakes

fun Widget.settleDown() { check(ready) }
`
	body := `    private val widget: Widget = Widget()

    @Test
    fun usesIt() {
        widget.settleDown()
    }`
	if n := crossFileFindings(t, helperPathTest, helper, body); n != 0 {
		t.Fatalf("expected 0 findings, got %d", n)
	}
}

func TestTestWithoutAssertion_UnknownReceiverNeedsAssertionLikeNameOrSingleDeclaration(t *testing.T) {
	twoOwners := `package fakes

class First { fun settleDown() { check(ready) } }
class Second { fun settleDown() { check(ready) } }
`
	body := `    @Test
    fun usesIt() {
        makeSomething().settleDown()
    }`
	if n := crossFileFindings(t, helperPathTest, twoOwners, body); n != 1 {
		t.Fatalf("expected 1 finding, got %d", n)
	}
	single := `package fakes

class First { fun settleDown() { check(ready) } }
`
	if n := crossFileFindings(t, helperPathTest, single, body); n != 0 {
		t.Fatalf("single declaration: expected 0 findings, got %d", n)
	}
	assertLike := `package fakes

class First { fun checkSettled() { check(ready) } }
class Second { fun checkSettled() { check(ready) } }
`
	assertBody := `    @Test
    fun usesIt() {
        makeSomething().checkSettled()
    }`
	if n := crossFileFindings(t, helperPathTest, assertLike, assertBody); n != 0 {
		t.Fatalf("assertion-like name: expected 0 findings, got %d", n)
	}
}

func TestTestWithoutAssertion_TestingModuleMainSourcesAreHelpers(t *testing.T) {
	findings := runTestWithoutAssertionOnProject(t, map[string]string{
		"lib/foo-testing/src/main/kotlin/fakes/AuditLog.kt": recordingFakeChecking,
		"app/src/test/kotlin/app/AuditTest.kt":              auditTestSource,
	})
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}
