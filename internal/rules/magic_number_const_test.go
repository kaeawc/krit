package rules_test

import (
	"strings"
	"testing"
)

type magicHit struct{ line, col int }

func magicHits(t *testing.T, code string) map[magicHit]bool {
	t.Helper()
	got := map[magicHit]bool{}
	for _, f := range runMagicNumberAsDetekt(t, code) {
		got[magicHit{f.Line, f.Col}] = true
	}
	return got
}

// lineCol returns the 1-based position of the first occurrence of needle.
func lineCol(t *testing.T, code, needle string) magicHit {
	t.Helper()
	off := strings.Index(code, needle)
	if off < 0 {
		t.Fatalf("%q not found", needle)
	}
	line := strings.Count(code[:off], "\n") + 1
	return magicHit{line, off - (strings.LastIndex(code[:off], "\n") + 1) + 1}
}

func TestMagicNumber_ConstDeclarationIsIgnoredWhereverTheLiteralSits(t *testing.T) {
	code := `package test
internal const val A = 5L
private const val B =
  7776000L // comment on the literal line
const val C: Long =
  30
const val D = E * 1000 / 5 + 3
class K {
  companion object {
    private const val F = 6000L
  }
}
object O {
  internal const val G = 7000L
}
@Suppress("x") const val H = 8000
val notConst =
  9000L
`
	got := magicHits(t, code)
	want := lineCol(t, code, "9000L")
	if len(got) != 1 || !got[want] {
		t.Errorf("only the literal of the non-const property should be reported at %v, got %v", want, got)
	}
}

func TestMagicNumber_ReportsAtTheLiteral(t *testing.T) {
	code := `package test
val a = listOf(
  4096,
  foo(8192),
)
fun f() = bar(1 + 777)
`
	got := magicHits(t, code)
	for _, needle := range []string{"4096", "8192", "777"} {
		if want := lineCol(t, code, needle); !got[want] {
			t.Errorf("expected a finding at %v for %s, got %v", want, needle, got)
		}
	}
}

func TestMagicNumber_CompanionOnlyIgnoresProperties(t *testing.T) {
	code := `package test
class K {
  companion object {
    val kept = 33
    val lazyOne: Int get() = 44
    fun helper() = foo(555)
    fun other(): Int {
      val local = 666
      return local
    }
  }
}
`
	got := magicHits(t, code)
	for _, needle := range []string{"555", "666"} {
		if want := lineCol(t, code, needle); !got[want] {
			t.Errorf("%s in a companion function must be reported, got %v", needle, got)
		}
	}
	for _, needle := range []string{"33", "44"} {
		if want := lineCol(t, code, needle); got[want] {
			t.Errorf("%s in a companion property must be ignored", needle)
		}
	}
}

func TestMagicNumber_ReturnIsOnlyIgnoredAsTheSoleStatement(t *testing.T) {
	code := `package test
fun sole(): Int { return 71 }
fun many(): Int {
  println()
  return 72
}
`
	got := magicHits(t, code)
	if want := lineCol(t, code, "71"); got[want] {
		t.Error("a function whose only statement returns a literal must be ignored")
	}
	if want := lineCol(t, code, "72"); !got[want] {
		t.Errorf("a returned literal after other statements must be reported, got %v", got)
	}
}

func TestMagicNumber_DefaultValueIsOnlyIgnoredWhenItIsTheLiteral(t *testing.T) {
	code := `package test
fun plain(x: Int = 5001) = x
fun computed(x: Int = 5002 * 3) = x
class C(val y: Int = 5003, val z: Int = -5004)
`
	got := magicHits(t, code)
	for _, needle := range []string{"5001", "5003"} {
		if want := lineCol(t, code, needle); got[want] {
			t.Errorf("default value %s must be ignored", needle)
		}
	}
	for _, needle := range []string{"5002", "3)", "5004"} {
		if want := lineCol(t, code, needle); !got[want] {
			t.Errorf("computed default %s must be reported, got %v", needle, got)
		}
	}
}
