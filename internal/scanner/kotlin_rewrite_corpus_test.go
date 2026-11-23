package scanner

import (
	"testing"
)

// Constructs from public Kotlin repositories that recovered with ERROR
// nodes (tests/corpus). Each case runs through TestKotlinRewriteCases and
// TestKotlinRewritePositionsMatchOriginal.
func init() {
	rewriteCases = append(rewriteCases, corpusRewriteCases...)
}

var corpusRewriteCases = []struct {
	name  string
	src   string
	check func(t *testing.T, ft *FlatTree, src []byte)
}{
	{
		// kotlinx.serialization, ktor, moshi, okhttp: hex digit decoding.
		name: "when branch after a line ending in a char literal",
		src:  "fun f(c: Char): Int = when (c) {\n    in '0'..'9' -> c - '0'\n    in 'A'..'F' -> c - 'A' + 10\n    else -> -1\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "when_entry"); len(got) != 3 || got[1] != "in 'A'..'F' -> c - 'A' + 10" {
				t.Errorf("when_entry = %q", got)
			}
		},
	},
	{
		// element-x-android presenters.
		name: "suspend lambda call",
		src:  "fun f() {\n    suspend {\n        load()\n    }.runCatchingUpdatingState(state)\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if p := parentTypeOf(ft, src, "simple_identifier", "suspend"); p != "call_expression" {
				t.Errorf("suspend parent = %q, want call_expression", p)
			}
		},
	},
	{
		name: "suspend function type is untouched",
		src:  "val a: suspend () -> Unit = {}\nval b: suspend() -> Unit = {}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "function_type"); len(got) != 2 {
				t.Errorf("function_type = %q", got)
			}
		},
	},
	{
		// okhttp Android10Platform, element-x analytics.
		name: "modifier word called as a function",
		src:  "fun f() {\n    guard.apply { open(closer) }\n    if (consent) open() else close()\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "call_expression"); len(got) != 4 {
				t.Errorf("call_expression = %q", got)
			}
		},
	},
	{
		// okhttp URLConnectionTest.
		name: "line-leading name starting with in and a digit",
		src:  "fun f() {\n    val in1 = open()\n    in1.close()\n    in_.close()\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "call_expression"); len(got) != 3 || got[1] != "in1.close()" {
				t.Errorf("call_expression = %q", got)
			}
		},
	},
	{
		// detekt LongParameterList, ktor CodeExecutionService.
		name: "property delegate on the next line",
		src:  "class A {\n    private val x: Int\n        by config(1)\n    val y by lazy { 2 }\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "property_delegate"); len(got) != 2 || got[0] != "by config(1)" {
				t.Errorf("property_delegate = %q", got)
			}
		},
	},
	{
		// leakcanary shark: getOrPut(k) { mutableListOf() } += v.
		name: "augmented assignment to a call with a trailing lambda",
		src:  "fun f(m: MutableMap<Int, MutableList<Int>>, k: Int, v: Int) {\n    m.getOrPut(k) { mutableListOf() } += v\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "assignment"); len(got) != 1 || got[0] != "m.getOrPut(k) { mutableListOf() } += v" {
				t.Errorf("assignment = %q", got)
			}
		},
	},
	{
		// detekt UseOrEmpty, okhttp Response.
		name: "return of a name starting with e in a when branch",
		src:  "fun f(x: Any): List<Int> {\n    when (x) {\n        is String -> return emptyList()\n        else -> return emptyList<Int>().plus(1)\n    }\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "jump_expression"); len(got) != 2 || got[0] != "return emptyList()" {
				t.Errorf("jump_expression = %q", got)
			}
		},
	},
	{
		// ktor, kotlinx.coroutines, kotlinx.serialization: KMP actual classes.
		name: "actual constructor property after another modifier",
		src:  "public actual class Frame actual constructor(\n    public actual val fin: Boolean,\n    @JvmField internal actual var job: Int,\n)\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "platform_modifier"); len(got) != 4 {
				t.Errorf("platform_modifier = %q, want four actual modifiers", got)
			}
			if got := nodeTexts(ft, src, "function_modifier"); len(got) != 0 {
				t.Errorf("function_modifier = %q, want none", got)
			}
			if got := nodeTexts(ft, src, "class_parameter"); len(got) != 2 {
				t.Errorf("class_parameter = %q", got)
			}
		},
	},
}

// The scanner's lexer counts character literal quotes as code, like string
// quotes, so a line ending in 'A' ends in code.
func TestKotlinLexerCharLiteralQuotesAreCode(t *testing.T) {
	src := []byte("val c = 'A'\n")
	lx := lexKotlin(src)
	open, closing := 8, 10
	if !lx.mask[open] || !lx.mask[closing] || lx.mask[open+1] {
		t.Errorf("mask = %v, want quotes as code and the character not", lx.mask[open:closing+1])
	}
}
