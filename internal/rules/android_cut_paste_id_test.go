package rules_test

import (
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func cutPasteLines(findings []scanner.Finding) []int {
	var lines []int
	for _, f := range findings {
		lines = append(lines, f.Line)
	}
	sort.Ints(lines)
	return lines
}

func TestCutPasteId_Java(t *testing.T) {
	const src = `package test;
import android.app.Activity;
import android.view.View;
public class A extends Activity {
    View field1, field2;
    void same() {
        View a = findViewById(R.id.name);
        View b = findViewById(R.id.name);
    }
    void sameVariable() {
        View v = findViewById(R.id.one);
        v = findViewById(R.id.one);
    }
    void otherReceiver(View p, View q) {
        View a = p.findViewById(R.id.two);
        View b = q.findViewById(R.id.two);
    }
    void branches(boolean c) {
        View a, b;
        if (c) { a = findViewById(R.id.three); } else { b = findViewById(R.id.three); }
    }
    void fields() {
        field1 = findViewById(R.id.four);
        field2 = findViewById(R.id.four);
    }
    void noVariable() {
        findViewById(R.id.five).setVisibility(View.GONE);
        findViewById(R.id.five).setVisibility(View.VISIBLE);
    }
    void third() {
        View a = findViewById(R.id.seven);
        View b = findViewById(R.id.seven);
        View c = findViewById(R.id.seven);
    }
    void different() {
        View a = findViewById(R.id.eight);
        View b = findViewById(R.id.nine);
    }
    void requireVariant() {
        View a = requireViewById(R.id.ten);
        View b = findViewById(R.id.ten);
    }
    void notAnIdConstant() {
        View a = findViewById(getId());
        View b = findViewById(getId());
    }
}
`
	findings := runRuleByNameOnJava(t, "CutPasteId", src)
	want := []int{8, 24, 32, 33, 41}
	got := cutPasteLines(findings)
	if len(got) != len(want) {
		t.Fatalf("expected findings on lines %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected findings on lines %v, got %v", want, got)
		}
	}
	if findings[0].Message != "The id `R.id.name` has already been looked up in this method; possible cut & paste error?" {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}

func TestCutPasteId_Kotlin(t *testing.T) {
	const src = `package test
import android.app.Activity
import android.view.View
class B : Activity() {
    var f1: View? = null
    var f2: View? = null
    fun same() {
        val a = findViewById<View>(R.id.name)
        val b = findViewById<View>(R.id.name)
    }
    fun cast() {
        val a = findViewById(R.id.cast) as View
        val b = findViewById(R.id.cast) as View
    }
    fun properties() {
        f1 = findViewById(R.id.k4)
        f2 = findViewById(R.id.k4)
    }
    fun locals() {
        var x: View? = null
        var y: View? = null
        x = findViewById(R.id.k7)
        y = findViewById(R.id.k7)
    }
    fun branches(c: Boolean) {
        val a: View?
        val b: View?
        if (c) { a = findViewById(R.id.k3) } else { b = findViewById(R.id.k3) }
    }
    fun sameVariable() {
        var v = findViewById<View>(R.id.k1)
        v = findViewById(R.id.k1)
    }
    fun otherReceiver(p: View, q: View) {
        val a = p.findViewById<View>(R.id.k2)
        val b = q.findViewById<View>(R.id.k2)
    }
    fun lambda() {
        val a = findViewById<View>(R.id.k6)
        run { val b = findViewById<View>(R.id.k6) }
    }
}
`
	got := cutPasteLines(runRuleByName(t, "CutPasteId", src))
	want := []int{9, 13, 23, 40}
	if len(got) != len(want) {
		t.Fatalf("expected findings on lines %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected findings on lines %v, got %v", want, got)
		}
	}
}
