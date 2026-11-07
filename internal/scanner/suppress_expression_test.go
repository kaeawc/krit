package scanner

import (
	"strings"
	"testing"
)

// expressionSuppressCase pairs a source with the probes that must and must not be suppressed.
type expressionSuppressCase struct {
	name    string
	src     string
	rule    string
	covered []string // substrings whose first occurrence sits inside the suppression
	exposed []string // substrings that must stay reported
}

func probePosition(t *testing.T, src, needle string) (line, col int) {
	t.Helper()
	off := strings.Index(src, needle)
	if off < 0 {
		t.Fatalf("%q not found", needle)
	}
	line = strings.Count(src[:off], "\n") + 1
	col = off - (strings.LastIndex(src[:off], "\n") + 1) + 1
	return line, col
}

func runExpressionSuppressCases(t *testing.T, cases []expressionSuppressCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sf := BuildSuppressionFilter(parsedFileForFilter(t, "X.kt", tc.src), nil, nil, "")
			for _, probe := range tc.covered {
				line, col := probePosition(t, tc.src, probe)
				if !sf.IsSuppressedAtColumn(tc.rule, "style", line, col) {
					t.Errorf("%s at %q (%d:%d) must be suppressed", tc.rule, probe, line, col)
				}
			}
			for _, probe := range tc.exposed {
				line, col := probePosition(t, tc.src, probe)
				if sf.IsSuppressedAtColumn(tc.rule, "style", line, col) {
					t.Errorf("%s at %q (%d:%d) must stay reported", tc.rule, probe, line, col)
				}
			}
		})
	}
}

func TestSuppressExpression_AnnotatedStatements(t *testing.T) {
	runExpressionSuppressCases(t, []expressionSuppressCase{
		{
			name:    "return with spread argument",
			rule:    "SpreadOperator",
			src:     "fun f(l: List<String>): String {\n  @Suppress(\"SpreadOperator\")\n  return join(\"a\", *l.toTypedArray())\n}\nfun g(l: List<String>) = join(\"b\", *l.toTypedArray())\n",
			covered: []string{"\"a\", *l"},
			exposed: []string{"\"b\", *l"},
		},
		{
			name:    "assignment",
			rule:    "SpreadOperator",
			src:     "fun f(l: List<String>) {\n  var y = \"\"\n  @Suppress(\"SpreadOperator\")\n  y = join(\"a\", *l.toTypedArray())\n  y = join(\"b\", *l.toTypedArray())\n}\n",
			covered: []string{"*l.toTypedArray())\n  y ="},
		},
		{
			name:    "local property initializer",
			rule:    "SpreadOperator",
			src:     "fun f(l: List<String>) {\n  @Suppress(\"SpreadOperator\")\n  val y = join(\"a\", *l.toTypedArray())\n  val z = join(\"b\", *l.toTypedArray())\n}\n",
			covered: []string{"*l.toTypedArray())\n  val z"},
		},
		{
			name:    "annotation stack",
			rule:    "SpreadOperator",
			src:     "fun f(l: List<String>) {\n  @Suppress(\"Other\")\n  @Deprecated(\"x\")\n  @Suppress(\"SpreadOperator\")\n  join(\"a\", *l.toTypedArray())\n}\n",
			covered: []string{"*l.toTypedArray()"},
		},
		{
			name:    "multiple names",
			rule:    "SpreadOperator",
			src:     "fun f(l: List<String>) {\n  @Suppress(\"Other\", \"SpreadOperator\")\n  join(\"a\", *l.toTypedArray())\n}\n",
			covered: []string{"*l.toTypedArray()"},
		},
		{
			name:    "names array",
			rule:    "SpreadOperator",
			src:     "fun f(l: List<String>) {\n  @Suppress(names = [\"Other\", \"SpreadOperator\"])\n  join(\"a\", *l.toTypedArray())\n}\n",
			covered: []string{"*l.toTypedArray()"},
		},
		{
			name:    "suppress warnings and detekt prefixes",
			rule:    "SpreadOperator",
			src:     "fun f(l: List<String>) {\n  @SuppressWarnings(\"SpreadOperator\")\n  join(\"a\", *l.toTypedArray())\n  @Suppress(\"detekt.SpreadOperator\")\n  join(\"b\", *l.toTypedArray())\n  @Suppress(\"detekt:SpreadOperator\")\n  join(\"c\", *l.toTypedArray())\n  @Suppress(\"style.SpreadOperator\")\n  join(\"d\", *l.toTypedArray())\n}\n",
			covered: []string{"\"a\", *l", "\"b\", *l", "\"c\", *l", "\"d\", *l"},
		},
		{
			name:    "same line annotation",
			rule:    "SpreadOperator",
			src:     "fun f(l: List<String>) {\n  @Suppress(\"SpreadOperator\") join(\"a\", *l.toTypedArray())\n  join(\"b\", *l.toTypedArray())\n}\n",
			covered: []string{"\"a\", *l"},
			exposed: []string{"\"b\", *l"},
		},
		{
			name:    "try expression",
			rule:    "SwallowedException",
			src:     "fun f(): Int {\n  @Suppress(\"SwallowedException\")\n  try {\n    return 1\n  } catch (e: IllegalStateException) {\n    return 2\n  }\n}\n",
			covered: []string{"catch (e"},
		},
		{
			name:    "returned try expression",
			rule:    "SwallowedException",
			src:     "fun f(): Int {\n  @Suppress(\"SwallowedException\")\n  return try {\n    1\n  } catch (e: IllegalStateException) {\n    2\n  }\n}\n",
			covered: []string{"catch (e"},
		},
		{
			name:    "expression body try",
			rule:    "SwallowedException",
			src:     "fun f(): Int? =\n  @Suppress(\"SwallowedException\")\n  try {\n    1\n  } catch (e: IllegalStateException) {\n    null\n  }\n",
			covered: []string{"catch (e"},
		},
		{
			name:    "property initializer try",
			rule:    "SwallowedException",
			src:     "val p: Int =\n  @Suppress(\"SwallowedException\")\n  try {\n    1\n  } catch (e: IllegalStateException) {\n    2\n  }\n",
			covered: []string{"catch (e"},
		},
		{
			name:    "file annotation",
			rule:    "SpreadOperator",
			src:     "@file:Suppress(\"SpreadOperator\")\n\npackage a\n\nfun f(l: List<String>) = join(\"a\", *l.toTypedArray())\n",
			covered: []string{"*l.toTypedArray()"},
		},
	})
}

// The grammar can end `@A return` at the keyword and parse the returned call as the next statement.
func TestSuppressExpression_AnnotatedReturnSplitFromOperand(t *testing.T) {
	runExpressionSuppressCases(t, []expressionSuppressCase{
		{
			name:    "spread first argument in class",
			rule:    "SpreadOperator",
			src:     "class P {\n  fun get(context: Context): CharSequence {\n    @Suppress(\"SpreadOperator\")\n    return expand(\n      *args.toTypedArray(),\n    )\n  }\n\n  fun other(): CharSequence {\n    return expand(\n      *args.toTypedArray(),\n    )\n  }\n}\n",
			covered: []string{"*args.toTypedArray(),\n    )\n  }\n\n  fun other"},
			exposed: []string{"*args.toTypedArray(),\n    )\n  }\n}"},
		},
		{
			name:    "spread first argument in lambda",
			rule:    "SpreadOperator",
			src:     "val f = run {\n  @Suppress(\"SpreadOperator\")\n  return@run expand(\n    *args.toTypedArray(),\n  )\n}\n",
			covered: []string{"*args.toTypedArray()"},
		},
	})
}

// The grammar can end an annotated expression body at its annotation and parse the expression as a sibling.
func TestSuppressExpression_AnnotatedExpressionBodySplit(t *testing.T) {
	runExpressionSuppressCases(t, []expressionSuppressCase{
		{
			name:    "try at end of file after documented function",
			rule:    "SwallowedException",
			src:     "fun Widget.attachAll(\n) {\n}\n/** Doc. */\nfun <F : Item> Holder.findItemOrNull(): F? =\n  @Suppress(\"SwallowedException\")\n  try {\n  } catch (e: IllegalStateException) {\n  }\n",
			covered: []string{"catch (e: IllegalStateException)"},
		},
	})
}
