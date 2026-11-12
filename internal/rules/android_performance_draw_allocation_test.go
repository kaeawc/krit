package rules_test

import "testing"

func TestDrawAllocation_ReceiversAndNestedClasses(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"clip bounds write is a setter", "child.clipBounds = null", 0},
		{"clip bounds read", "val b = child.clipBounds", 1},
		{"other receiver does not make the allocation lazy", "if (state.ready) { other.ready = true; val p = Paint() }", 1},
		{"qualified condition on another object", "if (state.ready) { ready = true; val p = Paint() }", 1},
		{"this-qualified lazy init", "if (this.cached == null) { this.cached = Paint() }", 0},
		{"lateinit lazy init", "if (!::lazyPaint.isInitialized) { lazyPaint = Paint() }", 0},
		{"call in condition is not a variable read", "if (isEmpty(cached)) { cached = Paint() }", 1},
		{"imported nested class constructor", "val o = BitmapFactory.Options()", 1},
		{"fully qualified nested class constructor", "val o = android.graphics.BitmapFactory.Options()", 1},
		{"same-file nested class constructor", "val s = V.Slot()", 1},
		{"unknown qualifier stays a member call", "val c = Colors.Primary()", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package test\nimport android.view.View\nimport android.graphics.BitmapFactory\nimport android.graphics.Canvas\nimport android.graphics.Paint\n" +
				"class State { var ready = false }\n" +
				"class V(c: android.content.Context) : View(c) {\n  class Slot\n  private var cached: Paint? = null\n  private lateinit var lazyPaint: Paint\n  private var ready = false\n" +
				"  private fun isEmpty(p: Paint?) = p == null\n" +
				"  private val state = State()\n  private val other = State()\n  private val child: View = this\n" +
				"  override fun onDraw(canvas: Canvas) {\n    " + tc.body + "\n  }\n}\n"
			findings := runRuleByName(t, "DrawAllocation", src)
			if len(findings) != tc.want {
				t.Fatalf("expected %d findings, got %d", tc.want, len(findings))
			}
		})
	}
}
