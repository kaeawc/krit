package rules_test

import "testing"

// #1612: T of mutableStateOf is also inferred through same-file generic
// lambda helpers and from the parameter a call passes it to.
func TestAutoboxingStateCreationInfersThroughHelpersAndArguments(t *testing.T) {
	const header = `package s

import androidx.compose.runtime.MutableState
import androidx.compose.runtime.State
import androidx.compose.runtime.mutableStateOf

inline fun <T> remember0(block: () -> T): T = block()
fun <T> Any.scoped(block: Any.() -> T): T = block()
fun <T> wrap(block: () -> T): List<T> = listOf(block())
fun concrete(block: () -> MutableState<Any>): MutableState<Any> = block()
fun use(any: Any?) {}
fun <T> take(value: T) {}
fun useInt(state: MutableState<Int>) {}
fun useAny(state: MutableState<Any>) {}
fun readInt(state: State<Int>) {}
fun useNumber(value: Number) {}
fun named(first: String, state: Any) {}
fun many(vararg states: Any) {}
fun overloaded(a: Any) {}
fun overloaded(a: Any, b: Int) {}
class Holder { fun use(any: Any?) {} }
`
	for _, test := range []struct {
		name, body string
		want       int
	}{
		{"generic lambda helper", "val t = remember0 { mutableStateOf(1) }", 1},
		{"helper result with declared MutableState<Any>", "val t: MutableState<Any> = remember0 { mutableStateOf(1) }", 0},
		{"helper result with declared MutableState<Int>", "val t: MutableState<Int> = remember0 { mutableStateOf(1) }", 1},
		{"Any? parameter", "use(mutableStateOf(1))", 1},
		{"type-parameter parameter", "take(mutableStateOf(1))", 1},
		{"MutableState<Int> parameter", "useInt(mutableStateOf(1))", 1},
		{"State<Int> parameter", "readInt(mutableStateOf(1))", 1},
		{"named Any argument", "named(state = mutableStateOf(1), first = \"x\")", 1},
		{"MutableState<Any> parameter", "useAny(mutableStateOf(1))", 0},
		{"non-generic helper fixes T", "val t = concrete { mutableStateOf(1) }", 0},
		{"helper returning List<T>", "val t = wrap { mutableStateOf(1) }", 0},
		{"Number parameter", "useNumber(mutableStateOf(1) as Number)", 0},
		{"vararg parameter", "many(mutableStateOf(1))", 0},
		{"overloaded callee", "overloaded(mutableStateOf(1))", 0},
		{"member call on a receiver", "Holder().use(mutableStateOf(1))", 0},
		{"String value", "use(mutableStateOf(\"x\"))", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			findings := runRuleByName(t, "AutoboxingStateCreation", header+"\nfun body() {\n    "+test.body+"\n}\n")
			if len(findings) != test.want {
				t.Fatalf("%s: expected %d findings, got %d: %v", test.body, test.want, len(findings), findings)
			}
		})
	}

	t.Run("local lookalike helper and callee", func(t *testing.T) {
		findings := runRuleByName(t, "AutoboxingStateCreation", `package s

import androidx.compose.runtime.mutableStateOf

fun body() {
    fun <T> remember0(block: () -> T): T = block()
    fun use(state: MutableStateLike) {}
    val t = remember0 { mutableStateOf(1) }
    use(mutableStateOf(1))
}

class MutableStateLike
`)
		// The local generic helper still infers T = Int; the local `use`
		// takes an unrelated type, so its argument is not inferred.
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})
}

// MutableCollectionMutableState reads the same type evidence.
func TestMutableCollectionMutableStateInfersFromArguments(t *testing.T) {
	findings := runRuleByName(t, "MutableCollectionMutableState", `package s

import androidx.compose.runtime.MutableState
import androidx.compose.runtime.mutableStateOf

fun use(any: Any?) {}
fun useList(state: MutableState<List<Int>>) {}

fun body() {
    use(mutableStateOf(mutableListOf(1)))
    useList(mutableStateOf(mutableListOf(1)))
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (the Any? argument), got %d: %v", len(findings), findings)
	}
}
