package rules_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func sparseArrayLines(findings []scanner.Finding) []int {
	var lines []int
	for _, f := range findings {
		lines = append(lines, f.Line)
	}
	return lines
}

// The expected lines are those Android Lint 32.4.1 reports for the same sources.
func TestUseSparseArrays_KotlinMatchesLint(t *testing.T) {
	findings := runRuleByName(t, "UseSparseArrays", `package p

import android.util.SparseArray

class C {
    fun f() {
        val a = SparseArray<Int>()
        val b = SparseArray<Boolean>()
        val c = SparseArray<String>()
        val d: SparseArray<Int> = SparseArray()
        val e = SparseArray<Int>(10)
        val f = SparseArray<Long>()
        val g = SparseArray<Integer>()
        val h = SparseArray<Int?>()
        val i = HashMap<Int, String>()
        val j = hashMapOf<Int, Boolean>()
    }
}
`)
	if got, want := sparseArrayLines(findings), []int{7, 8, 11, 13, 14}; !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %v, want %v", got, want)
	}
	if !strings.Contains(findings[0].Message, "SparseIntArray") || !strings.Contains(findings[1].Message, "SparseBooleanArray") {
		t.Fatalf("unexpected suggestions: %q, %q", findings[0].Message, findings[1].Message)
	}
}

func TestUseSparseArrays_KotlinRequiresFrameworkClass(t *testing.T) {
	cases := map[string]struct {
		code string
		want int
	}{
		"no import": {`package p
class C { val a = SparseArray<Int>() }
`, 0},
		"other package": {`package p
import com.example.SparseArray
class C { val a = SparseArray<Int>() }
`, 0},
		"star import": {`package p
import android.util.*
class C { val a = SparseArray<Int>() }
`, 1},
		"qualified": {`package p
class C { val a = android.util.SparseArray<Boolean>() }
`, 1},
		"compat": {`package p
import androidx.collection.SparseArrayCompat
class C { val a = SparseArrayCompat<Int>() }
`, 0},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := len(runRuleByName(t, "UseSparseArrays", tc.code)); got != tc.want {
				t.Fatalf("findings = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestUseSparseArrays_JavaMatchesLint(t *testing.T) {
	findings := runRuleByNameOnJava(t, "UseSparseArrays", `package p;

import android.util.SparseArray;

public class D {
    void f() {
        SparseArray<Integer> a = new SparseArray<Integer>();
        SparseArray<Boolean> b = new SparseArray<Boolean>();
        SparseArray<String> c = new SparseArray<String>();
        SparseArray<Integer> d = new SparseArray<>();
        SparseArray<Integer> e = new SparseArray<Integer>(10);
        SparseArray<Long> f = new SparseArray<Long>();
        SparseArray g = new SparseArray();
        SparseArray<String> h = new SparseArray<>();
        java.util.HashMap<Integer, String> i = new java.util.HashMap<Integer, String>();
    }
}
`)
	if got, want := sparseArrayLines(findings), []int{7, 8, 10, 11}; !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %v, want %v", got, want)
	}
	if !strings.Contains(findings[0].Message, "new SparseIntArray") {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}
