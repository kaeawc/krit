package rules_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestUnsafeCallOnNullableType_SourceNonNull(t *testing.T) {
	tests := []struct {
		name string
		code string
		want int
	}{
		{"explicit local", `fun f() { val x: String = "hello"; val y = x!! }`, 0},
		{"explicit var", `fun f() { var x: String = "hello"; println(x!!) }`, 0},
		{"do while body shadow", `fun f(value: String) { do { val value: String? = next() } while (value!!.isEmpty()) }`, 1},
		{"declared parameter", `class KnownThing; fun f(value: KnownThing) { println(value!!) }`, 0},
		{"string parameter", `fun f(value: String) { println(value!!) }`, 0},
		{"receiver this cases", `class Formatter {
    fun String?.orFail(): String = this!!
    fun g(s: String?) = s.run { this!!.length }
    fun h(s: String?) = with(s) { this!!.length }
}`, 3},
		{"object literal member shadow", `fun f(value: String) = object : Runnable { val value: String? = null; override fun run() { println(value!!) } }`, 1},
		{"when subject shadow", `fun f(value: String) { when (val value = external()) { else -> println(value!!) } }`, 1},
		{"nullable local", `fun f() { val x: String? = null; println(x!!) }`, 1},
		{"parenthesized nullable local after assignment", `fun f(y: String) { var x: (String?) = y; x = null; println(x!!) }`, 1},
		{"function type local", `fun f() { val x: () -> String = { "hello" }; println(x!!) }`, 1},
		{"nullable parameter", `fun f(value: String?) { println(value!!) }`, 1},
		{"safe call", `fun f(value: String?) { println(value?.trim()!!) }`, 1},
		{"inferred safe call", `fun f(value: String?) { val x = value?.trim(); println(x!!) }`, 1},
		{"generic parameter", `fun <T> f(value: T) { println(value!!) }`, 1},
		{"bounded generic", `fun <T: Any> f(value: T) { println(value!!) }`, 1},
		{"generic local", `fun <T> f(value: T) { val x: T = value; println(x!!) }`, 1},
		{"inferred generic local", `fun <T> f(value: T) { val x = value; println(x!!) }`, 1},
		{"generic call", `fun <T> identity(value: T): T = value; fun <T> f(value: T) { println(identity(value)!!) }`, 1},
		{"nullable call", `fun name(): String? = null; fun f() { println(name()!!) }`, 1},
		{"unknown call", `fun f() { println(external()!!) }`, 1},
		{"platform call", `fun f() { println(System.getProperty("key")!!) }`, 1},
		{"platform local", `fun f() { val x = System.getProperty("key"); println(x!!) }`, 1},
		{"platform var", `fun f() { var x = System.getProperty("key"); println(x!!) }`, 1},
		{"nullable shadow", `fun f(value: String) { val value: String? = null; println(value!!) }`, 1},
		{"unknown shadow", `fun f(value: String) { val value = external(); println(value!!) }`, 1},
		{"block shadow", `fun f(value: String) { if (true) { val value: String? = null; println(value!!) } }`, 1},
		{"sibling block", `fun f(value: String?) { if (true) { val value: String = "hello" }; println(value!!) }`, 1},
		{"lambda shadow", `fun f(value: String) { listOf<String?>(null).forEach { value -> println(value!!) } }`, 1},
		{"destructured shadow", `fun f(value: String) { val (other, value) = external(); println(value!!) }`, 1},
		{"shadow initializer", `fun f(value: String?) { val value: String = value!! }`, 1},
		{"loop shadow", `fun f(value: String, values: List<String?>) { for (value in values) println(value!!) }`, 1},
		{"catch shadow", `fun f(value: String) { try { external() } catch (value: Missing) { println(value!!) } }`, 1},
		{"nullable typealias", `typealias Name = String?; fun f(value: Name) { println(value!!) }`, 1},
		{"imported alias", "import example.Name as Label\nfun f(value: Label) { println(value!!) }", 1},
		{"import uncertainty", "import example.Name\nfun f(value: Name) { println(value!!) }", 1},
		{"wildcard uncertainty", "import example.*\nfun f(value: String) { println(value!!) }", 1},
		{"unresolved annotation", `fun f(value: Missing) { println(value!!) }`, 1},
		{"out of scope class", `class Outer { class Missing }; fun f(value: Missing) { println(value!!) }`, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, resolver := range []bool{false, true} {
				findings := runRuleByName(t, "UnsafeCallOnNullableType", test.code)
				if resolver {
					findings = runRuleByNameWithResolver(t, "UnsafeCallOnNullableType", test.code)
				}
				if len(findings) != test.want {
					t.Errorf("resolver=%v: want %d findings, got %+v", resolver, test.want, findings)
				}
			}
		})
	}
}

func TestUnsafeCallOnNullableType_ConservativelyReportsUntypedIdentifiers(t *testing.T) {
	for _, code := range []string{
		`fun f() { val x = "hello"; println(x!!) }`,
		`fun name(): String = "hello"; fun f() { println(name()!!) }`,
	} {
		for _, resolver := range []bool{false, true} {
			findings := runRuleByName(t, "UnsafeCallOnNullableType", code)
			if resolver {
				findings = runRuleByNameWithResolver(t, "UnsafeCallOnNullableType", code)
			}
			if len(findings) != 1 {
				t.Errorf("resolver=%v: expected a finding when source syntax does not prove a simple identifier declaration, got %+v", resolver, findings)
			}
		}
	}
}

func TestUnsafeCallOnNullableType_Fixtures(t *testing.T) {
	for _, fixture := range []struct {
		kind string
		name string
		want bool
	}{
		{"positive", "UnsafeCallOnNullableType", true},
		{"negative", "UnsafeCallOnNullableType", false},
		{"positive", "UnnecessaryNotNullOperator", false},
	} {
		path := filepath.Join("..", "..", "tests", "fixtures", fixture.kind, "potential-bugs", fixture.name+".kt")
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		findings := runRuleByNameOnFile(t, "UnsafeCallOnNullableType", file)
		if (len(findings) > 0) != fixture.want {
			t.Errorf("%s: findings=%+v", path, findings)
		}
	}
}
