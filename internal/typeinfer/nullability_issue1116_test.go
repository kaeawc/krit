package typeinfer

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// resolveProbeInitializer resolves the initializer of the local `val <probe> = expr` in src.
func resolveProbeInitializer(t *testing.T, src, probe string) *ResolvedType {
	t.Helper()
	file := parseTestFile(t, src)
	r := NewResolver()
	r.IndexFilesParallel([]*scanner.File{file}, 1)
	var init uint32
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if init != 0 || file.FlatType(idx) != "property_declaration" {
			return
		}
		varDecl := flatFindNamedChildOfType(file, idx, "variable_declaration")
		if varDecl == 0 || flatFirstIdentifierText(file, varDecl) != probe {
			return
		}
		init = file.FlatNamedChild(idx, file.FlatNamedChildCount(idx)-1)
	})
	if init == 0 {
		t.Fatalf("no `val %s = ...` in source", probe)
	}
	got := r.ResolveFlatNode(init, file)
	if got == nil {
		t.Fatalf("%s: resolver returned nil", probe)
	}
	return got
}

// assertProvenNonNull fails unless got is a resolved, non-null type named name.
func assertProvenNonNull(t *testing.T, probe string, got *ResolvedType, name string) {
	t.Helper()
	if got.Kind == TypeUnknown || !got.Resolved || got.IsNullable() || got.Name != name {
		t.Errorf("%s: want non-null %s, got %+v", probe, name, got)
	}
}

// assertNotProvenNonNull fails when got claims a known non-null type.
func assertNotProvenNonNull(t *testing.T, probe string, got *ResolvedType) {
	t.Helper()
	if got.Kind != TypeUnknown && got.Resolved && !got.IsNullable() {
		t.Errorf("%s: want nullable or unknown, got proven non-null %+v", probe, got)
	}
}

func TestNullCheckNarrowing_OnlyWholeConditionOrDisjunctionOfEqualsNull(t *testing.T) {
	src := `
fun f(x: String?, flag: Boolean) {
    if (x != null || flag) {
        val orBody = x
    }
    if ((x != null) || flag) {
        val parenOrBody = x
    }
    if (flag || x != null) {
        val orRightBody = x
    }
    if (check(x != null)) {
        val argBody = x
    }
    if ((x != null)) {
        val parenBody = x
    }
}

fun g(x: String?, flag: Boolean) {
    if (x == null || flag) return
    val afterOrEarlyExit = x
}

fun h(x: String?) {
    if (!(x == null)) return
    val afterNegatedEarlyExit = x
}

fun check(b: Boolean): Boolean = b
`
	for _, probe := range []string{"orBody", "parenOrBody", "orRightBody", "argBody", "afterNegatedEarlyExit"} {
		assertNotProvenNonNull(t, probe, resolveProbeInitializer(t, src, probe))
	}
	// Sound narrowing is kept: a lone (parenthesized) `x != null` holds in the then-branch, and every
	// `x == null` operand of a `||` is false after the early exit.
	for _, probe := range []string{"parenBody", "afterOrEarlyExit"} {
		assertProvenNonNull(t, probe, resolveProbeInitializer(t, src, probe), "String")
	}
}

func TestExtensionReceiverDeclaresThis(t *testing.T) {
	src := `
class Config(val name: String)

class Overlay {
    fun Config?.nullableReceiver() {
        val nullableThis = this
    }

    fun Config.nonNullReceiver() {
        val nonNullThis = this
    }

    fun member() {
        val memberThis = this
    }

    val Config?.inlineGetter: Int get() {
        val inlineGetterThis = this
        return 0
    }

    val Config?.siblingGetter: Int
        get() {
            val siblingGetterThis = this
            return 0
        }
}

fun <T> T.genericReceiver() {
    val genericThis = this
}
`
	for _, probe := range []string{"nullableThis", "inlineGetterThis", "siblingGetterThis"} {
		got := resolveProbeInitializer(t, src, probe)
		if !got.IsNullable() || got.Name != "Config" {
			t.Errorf("%s: want Config?, got %+v", probe, got)
		}
	}
	assertProvenNonNull(t, "nonNullThis", resolveProbeInitializer(t, src, "nonNullThis"), "Config")
	assertProvenNonNull(t, "memberThis", resolveProbeInitializer(t, src, "memberThis"), "Overlay")
	if got := resolveProbeInitializer(t, src, "genericThis"); got.Name == "Overlay" {
		t.Errorf("genericThis: `this` of a top-level extension must not be a class, got %+v", got)
	}
}

func TestStdlibElementReturnKeepsElementNullability(t *testing.T) {
	src := `
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.catch
import kotlinx.coroutines.flow.first

class Channel(val name: String)

interface Repo {
    fun byId(id: String): Flow<Channel?>
    fun all(): Flow<Channel>
}

suspend fun f(repo: Repo) {
    val nullableElement = repo.byId("a").first()
    val lostElement = repo.byId("a").catch { emit(null) }.first()
    val nonNullElement = repo.all().first()
    val literalList = listOf(1, 2).first()
    val mixedList = listOf(1, "a").first()
    val nullInList = listOf(1, null).first()
    val emptyFactory = listOf<String?>().first()
    val firstOrNull = listOf(1).firstOrNull()
    val mapValue = mapOf(1 to Channel("a"), 2 to Channel("b")).getValue(1)
    val mixedMapValue = mapOf(1 to Channel("a"), 2 to "b").getValue(1)
}
`
	for _, probe := range []string{"nullableElement", "lostElement", "mixedList", "nullInList", "emptyFactory", "firstOrNull", "mixedMapValue"} {
		assertNotProvenNonNull(t, probe, resolveProbeInitializer(t, src, probe))
	}
	assertProvenNonNull(t, "nonNullElement", resolveProbeInitializer(t, src, "nonNullElement"), "Channel")
	assertProvenNonNull(t, "literalList", resolveProbeInitializer(t, src, "literalList"), "Int")
	assertProvenNonNull(t, "mapValue", resolveProbeInitializer(t, src, "mapValue"), "Channel")
}

func TestExtensionReceiverMemberShadowsOuterNames(t *testing.T) {
	src := `
class Holder(val label: String?)

class Outer(private val label: String, private val count: Int) {
    fun Holder.ext(param: String) {
        val outerName = label
        val ownParam = param
        val local = "x"
        val ownLocal = local
    }

    fun member() {
        val memberSeesOuter = label
    }
}
`
	if got := resolveProbeInitializer(t, src, "outerName"); got.Kind != TypeUnknown {
		t.Errorf("outerName: `label` inside `fun Holder.ext()` may be Holder's member, want unknown, got %+v", got)
	}
	assertProvenNonNull(t, "ownParam", resolveProbeInitializer(t, src, "ownParam"), "String")
	assertProvenNonNull(t, "ownLocal", resolveProbeInitializer(t, src, "ownLocal"), "String")
	assertProvenNonNull(t, "memberSeesOuter", resolveProbeInitializer(t, src, "memberSeesOuter"), "String")
}
