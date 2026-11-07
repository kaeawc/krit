package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	rulespkg "github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestHardcodedValuesResource(t *testing.T) {
	r := findResourceRule(t, "HardcodedValuesResource")

	t.Run("hardcoded text triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Text: "Hello World",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "Hello World",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("string reference is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Text: "@string/hello",
			Line: 5,
			Attributes: map[string]string{
				"android:text": "@string/hello",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("hardcoded hint triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "EditText",
			Line: 3,
			Attributes: map[string]string{
				"android:hint": "Type here",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("empty layout is clean", func(t *testing.T) {
		idx := emptyIndex()
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// MissingContentDescriptionResource
// ---------------------------------------------------------------------------

func TestPxUsageResource(t *testing.T) {
	r := findResourceRule(t, "PxUsageResource")

	t.Run("px in layout_width triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_width": "100px",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("dp in layout_width is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_width": "100dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("px in dimens values triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Dimensions["margin_large"] = "24px"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// NestedScrollingResource
// ---------------------------------------------------------------------------

func TestInefficientWeightResource(t *testing.T) {
	r := findResourceRule(t, "InefficientWeightResource")
	run := func(orientation, width, height, weight string) []scanner.Finding {
		parentAttrs := map[string]string{}
		if orientation != "" {
			parentAttrs["android:orientation"] = orientation
		}
		child := &android.View{Type: "TextView", Line: 3, Attributes: map[string]string{
			"android:layout_width": width, "android:layout_height": height, "android:layout_weight": weight}}
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout", Line: 1, Attributes: parentAttrs, Children: []*android.View{child}})
		return runResourceRule(r, idx)
	}

	t.Run("horizontal weight with wrap_content width triggers", func(t *testing.T) {
		if got := run("horizontal", "wrap_content", "wrap_content", "1"); len(got) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(got))
		}
	})
	t.Run("default orientation is horizontal", func(t *testing.T) {
		if got := run("", "match_parent", "wrap_content", "1"); len(got) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(got))
		}
	})
	t.Run("vertical weight with wrap_content height triggers", func(t *testing.T) {
		if got := run("vertical", "match_parent", "wrap_content", "2"); len(got) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(got))
		}
	})
	t.Run("0dp along the layout direction is clean", func(t *testing.T) {
		if got := run("horizontal", "0dp", "wrap_content", "1"); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(got))
		}
		if got := run("vertical", "wrap_content", "0dp", "1"); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(got))
		}
	})
	t.Run("any layout_weight value counts, as in Lint", func(t *testing.T) {
		if got := run("horizontal", "wrap_content", "wrap_content", "0"); len(got) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(got))
		}
	})
	t.Run("message names the attribute and the current size", func(t *testing.T) {
		got := run("vertical", "match_parent", "@dimen/h", "1")
		want := "Use a `layout_height` of `0dp` instead of `@dimen/h` for better performance"
		if len(got) != 1 || got[0].Message != want {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("two weighted children are clean", func(t *testing.T) {
		attrs := map[string]string{"android:layout_width": "wrap_content", "android:layout_weight": "1"}
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout", Line: 1, Attributes: map[string]string{},
			Children: []*android.View{
				{Type: "TextView", Line: 2, Attributes: attrs},
				{Type: "TextView", Line: 3, Attributes: attrs},
			}})
		if got := runResourceRule(r, idx); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(got))
		}
	})
	t.Run("undefined size is reported as (undefined)", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout", Line: 1, Attributes: map[string]string{},
			Children: []*android.View{{Type: "TextView", Line: 2, Attributes: map[string]string{"android:layout_weight": "1"}}}})
		got := runResourceRule(r, idx)
		if len(got) != 1 || !strings.Contains(got[0].Message, "instead of `(undefined)`") {
			t.Fatalf("got %+v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// DisableBaselineAlignmentResource
// ---------------------------------------------------------------------------

func TestDisableBaselineAlignmentResource(t *testing.T) {
	r := findResourceRule(t, "DisableBaselineAlignmentResource")
	weighted := func(typ string) *android.View {
		return &android.View{Type: typ, Line: 3, Attributes: map[string]string{"android:layout_weight": "1"}}
	}
	linear := func(attrs map[string]string, children ...*android.View) *android.ResourceIndex {
		return indexWithLayout("main", "res/layout/main.xml", &android.View{Type: "LinearLayout", Line: 1, Col: 1, Attributes: attrs, Children: children})
	}

	t.Run("weighted nested layouts without baselineAligned trigger", func(t *testing.T) {
		got := runResourceRule(r, linear(map[string]string{}, weighted("LinearLayout"), weighted("FrameLayout")))
		if len(got) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(got))
		}
		if got[0].Message != "Set `android:baselineAligned=\"false\"` on this element for better performance" {
			t.Fatalf("unexpected message: %s", got[0].Message)
		}
	})

	t.Run("fragment include and custom Layout children count as layouts", func(t *testing.T) {
		for _, typ := range []string{"fragment", "include", "ScrollView", "com.example.MyLayout", "androidx.constraintlayout.widget.ConstraintLayout"} {
			if got := runResourceRule(r, linear(map[string]string{}, weighted(typ))); len(got) != 1 {
				t.Errorf("%s: expected 1 finding, got %d", typ, len(got))
			}
		}
	})

	t.Run("a non-layout child keeps baseline alignment useful", func(t *testing.T) {
		for _, typ := range []string{"TextView", "Button", "RadioGroup", "Space", "ViewStub", "androidx.cardview.widget.CardView", "androidx.fragment.app.FragmentContainerView"} {
			if got := runResourceRule(r, linear(map[string]string{}, weighted("LinearLayout"), weighted(typ))); len(got) != 0 {
				t.Errorf("%s: expected 0 findings, got %d", typ, len(got))
			}
		}
	})

	t.Run("any baselineAligned attribute is clean", func(t *testing.T) {
		for _, value := range []string{"false", "true", "@bool/aligned"} {
			if got := runResourceRule(r, linear(map[string]string{"android:baselineAligned": value}, weighted("LinearLayout"))); len(got) != 0 {
				t.Errorf("baselineAligned=%q: expected 0 findings, got %d", value, len(got))
			}
		}
	})

	t.Run("vertical layouts without weights and non LinearLayout parents are clean", func(t *testing.T) {
		if got := runResourceRule(r, linear(map[string]string{"android:orientation": "vertical"}, weighted("LinearLayout"))); len(got) != 0 {
			t.Errorf("vertical: expected 0 findings, got %d", len(got))
		}
		noWeight := &android.View{Type: "LinearLayout", Line: 3, Attributes: map[string]string{}}
		if got := runResourceRule(r, linear(map[string]string{}, noWeight)); len(got) != 0 {
			t.Errorf("no weight: expected 0 findings, got %d", len(got))
		}
		frame := indexWithLayout("main", "res/layout/main.xml", &android.View{Type: "FrameLayout", Line: 1, Attributes: map[string]string{}, Children: []*android.View{weighted("LinearLayout")}})
		if got := runResourceRule(r, frame); len(got) != 0 {
			t.Errorf("FrameLayout parent: expected 0 findings, got %d", len(got))
		}
	})

	t.Run("a weight of zero still counts as a weight attribute", func(t *testing.T) {
		zero := &android.View{Type: "LinearLayout", Line: 3, Attributes: map[string]string{"android:layout_weight": "0"}}
		if got := runResourceRule(r, linear(map[string]string{}, zero)); len(got) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(got))
		}
	})
}

// ---------------------------------------------------------------------------
// NestedWeightsResource
// ---------------------------------------------------------------------------

func TestNestedWeightsResource(t *testing.T) {
	r := findResourceRule(t, "NestedWeightsResource")

	t.Run("nested LinearLayouts both with weights triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "LinearLayout",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_weight": "1",
					},
					Children: []*android.View{
						{
							Type: "TextView",
							Line: 5,
							Attributes: map[string]string{
								"android:layout_weight": "1",
							},
						},
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("single level weights is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_weight": "1",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// UselessLeafResource
// ---------------------------------------------------------------------------

func TestRequiredSizeResource(t *testing.T) {
	r := findResourceRule(t, "RequiredSizeResource")

	t.Run("view missing both width and height triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "TextView",
			Line:       5,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("view missing only width triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_height": "wrap_content",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("view with both width and height is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_width":  "match_parent",
				"android:layout_height": "wrap_content",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("data binding directive tags are clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "layout",
			Line: 1,
			Children: []*android.View{
				{
					Type: "data",
					Line: 2,
					Children: []*android.View{
						{Type: "import", Line: 3, Attributes: map[string]string{}},
						{Type: "variable", Line: 4, Attributes: map[string]string{}},
					},
				},
				{
					Type: "TextView",
					Line: 6,
					Attributes: map[string]string{
						"android:layout_width":  "match_parent",
						"android:layout_height": "wrap_content",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// SpUsageResource
// ---------------------------------------------------------------------------

func TestSpUsageResource(t *testing.T) {
	r := findResourceRule(t, "SpUsageResource")

	t.Run("textSize with dp triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textSize": "14dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("textSize with sp is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textSize": "14sp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("textSize with dip triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textSize": "14dip",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// TextFieldsResource
// ---------------------------------------------------------------------------

func TestNegativeMarginResource(t *testing.T) {
	r := findResourceRule(t, "NegativeMarginResource")

	t.Run("negative margin triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{{
				Type: "TextView",
				Line: 5,
				Attributes: map[string]string{
					"android:layout_marginTop": "-8dp",
				},
			}},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative margin under overlap-friendly or unclipped parent is clean", func(t *testing.T) {
		for name, parent := range map[string]*android.View{
			"frame":        {Type: "FrameLayout", Attributes: map[string]string{}},
			"constraint":   {Type: "androidx.constraintlayout.widget.ConstraintLayout", Attributes: map[string]string{}},
			"relative":     {Type: "RelativeLayout", Attributes: map[string]string{}},
			"noClip":       {Type: "LinearLayout", Attributes: map[string]string{"android:clipChildren": "false"}},
			"rootNoParent": nil,
		} {
			t.Run(name, func(t *testing.T) {
				child := &android.View{
					Type:       "TextView",
					Line:       5,
					Attributes: map[string]string{"android:layout_marginTop": "-8dp"},
				}
				root := child
				if parent != nil {
					parent.Line = 1
					parent.Children = []*android.View{child}
					root = parent
				}
				findings := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root))
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings, got %d", len(findings))
				}
			})
		}
	})

	t.Run("tools ignore suppresses negative margin", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{"tools:ignore": "NegativeMargin"},
			Children: []*android.View{{
				Type:       "TextView",
				Line:       5,
				Attributes: map[string]string{"android:layout_marginTop": "-8dp"},
			}},
		})
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("positive margin is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_marginTop": "8dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("multiple negative margins trigger single combined finding", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{{
				Type: "TextView",
				Line: 5,
				Attributes: map[string]string{
					"android:layout_marginTop":  "-8dp",
					"android:layout_marginLeft": "-4dp",
				},
			}},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 combined finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "layout_marginTop") ||
			!strings.Contains(findings[0].Message, "layout_marginLeft") {
			t.Fatalf("expected combined message mentioning both attributes, got %q", findings[0].Message)
		}
	})

	t.Run("project allowlist suppresses matching negative margins", func(t *testing.T) {
		rule := r.Implementation.(*rulespkg.NegativeMarginResourceRule)
		orig := append([]string(nil), rule.AllowedNegativeMargins...)
		rule.AllowedNegativeMargins = []string{
			"TextView:android:layout_marginTop=-8dp",
			"android:layout_marginLeft=-4dp",
		}
		defer func() { rule.AllowedNegativeMargins = orig }()

		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_marginTop":  "-8dp",
				"android:layout_marginLeft": "-4dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected allowlisted margins to be clean, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// RtlHardcodedResource
// ---------------------------------------------------------------------------

func TestSmallSpResource(t *testing.T) {
	r := findResourceRule(t, "SmallSpResource")

	t.Run("textSize 8sp triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textSize": "8sp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "8sp") {
			t.Fatalf("expected message to mention 8sp, got: %s", findings[0].Message)
		}
	})

	t.Run("textSize 11sp is clean, Lint's threshold is below 11sp", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textSize": "11sp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("textSize 12sp is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textSize": "12sp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("textSize 16sp is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textSize": "16sp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("textSize in dp is clean (not sp)", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textSize": "8dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty index is clean", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// Suspicious0dpResource
// ---------------------------------------------------------------------------

func TestSuspicious0dpResource(t *testing.T) {
	r := findResourceRule(t, "Suspicious0dpResource")

	zeroDpChild := func(typ, orientation string, attrs map[string]string) *android.ResourceIndex {
		child := &android.View{Type: typ, Line: 3, Attributes: attrs, LayoutWidth: attrs["android:layout_width"], LayoutHeight: attrs["android:layout_height"]}
		parentAttrs := map[string]string{}
		if orientation != "" {
			parentAttrs["android:orientation"] = orientation
		}
		return indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout", Line: 1, Attributes: parentAttrs, Children: []*android.View{child},
		})
	}

	t.Run("0dp width without weight in horizontal LinearLayout triggers", func(t *testing.T) {
		findings := runResourceRule(r, zeroDpChild("TextView", "horizontal", map[string]string{
			"android:layout_width": "0dp", "android:layout_height": "wrap_content",
		}))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if want := "Suspicious size: this will make the view invisible, should be used with `layout_weight`"; findings[0].Message != want {
			t.Fatalf("message = %q, want %q", findings[0].Message, want)
		}
		if findings[0].Severity != "error" {
			t.Fatalf("severity = %q, want error", findings[0].Severity)
		}
	})

	t.Run("0dp width without orientation is horizontal and triggers", func(t *testing.T) {
		findings := runResourceRule(r, zeroDpChild("TextView", "", map[string]string{
			"android:layout_width": "0dp", "android:layout_height": "wrap_content",
		}))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("0dp height without weight in vertical LinearLayout triggers", func(t *testing.T) {
		findings := runResourceRule(r, zeroDpChild("TextView", "vertical", map[string]string{
			"android:layout_width": "wrap_content", "android:layout_height": "0dp",
		}))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "should be used with `layout_weight`") {
			t.Fatalf("unexpected message: %s", findings[0].Message)
		}
	})

	t.Run("0dp across the orientation axis is invisible too", func(t *testing.T) {
		// Without a weight, Lint reports a 0 size on either axis.
		for _, tc := range []struct{ orientation, w, h string }{
			{"vertical", "0dp", "wrap_content"},
			{"horizontal", "wrap_content", "0dp"},
		} {
			findings := runResourceRule(r, zeroDpChild("TextView", tc.orientation, map[string]string{
				"android:layout_width": tc.w, "android:layout_height": tc.h,
			}))
			if len(findings) != 1 {
				t.Fatalf("%s: expected 1 finding, got %d", tc.orientation, len(findings))
			}
		}
	})

	t.Run("weighted 0dp on the wrong axis suggests the other dimension", func(t *testing.T) {
		findings := runResourceRule(r, zeroDpChild("TextView", "vertical", map[string]string{
			"android:layout_width": "0dp", "android:layout_height": "wrap_content", "android:layout_weight": "1",
		}))
		if len(findings) != 1 || findings[0].Message != "Suspicious size: this will make the view invisible, probably intended for `layout_height`" {
			t.Fatalf("unexpected findings: %+v", findings)
		}
		findings = runResourceRule(r, zeroDpChild("TextView", "", map[string]string{
			"android:layout_width": "wrap_content", "android:layout_height": "0px", "android:layout_weight": "1",
		}))
		if len(findings) != 1 || findings[0].Message != "Suspicious size: this will make the view invisible, probably intended for `layout_width`" {
			t.Fatalf("unexpected findings: %+v", findings)
		}
	})

	t.Run("a custom view or a child without a 0 size stops the scan, as in Lint", func(t *testing.T) {
		bad := &android.View{Type: "TextView", Line: 5, Attributes: map[string]string{"android:layout_width": "0dp", "android:layout_height": "wrap_content"}}
		for _, first := range []*android.View{
			{Type: "com.acme.Custom", Line: 3, Attributes: map[string]string{}},
			{Type: "TextView", Line: 3, Attributes: map[string]string{"android:layout_width": "wrap_content", "android:layout_height": "wrap_content"}},
		} {
			idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
				Type: "LinearLayout", Line: 1, Attributes: map[string]string{}, Children: []*android.View{first, bad},
			})
			if findings := runResourceRule(r, idx); len(findings) != 0 {
				t.Fatalf("expected 0 findings after %s, got %d", first.Type, len(findings))
			}
		}
	})

	t.Run("Space with a fixed size along the axis is a spacer", func(t *testing.T) {
		findings := runResourceRule(r, zeroDpChild("Space", "vertical", map[string]string{
			"android:layout_width": "0dp", "android:layout_height": "37.5dp",
		}))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("Space is never flagged", func(t *testing.T) {
		findings := runResourceRule(r, zeroDpChild("Space", "vertical", map[string]string{
			"android:layout_width": "wrap_content", "android:layout_height": "0dp",
		}))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("0dp width in horizontal LinearLayout is clean (correct usage)", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:orientation": "horizontal",
			},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_width":  "0dp",
						"android:layout_height": "wrap_content",
						"android:layout_weight": "1",
					},
					LayoutWidth:  "0dp",
					LayoutHeight: "wrap_content",
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("0dp height in vertical LinearLayout is clean (correct usage)", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:orientation": "vertical",
			},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_width":  "wrap_content",
						"android:layout_height": "0dp",
						"android:layout_weight": "1",
					},
					LayoutWidth:  "wrap_content",
					LayoutHeight: "0dp",
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty index is clean", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// InOrMmUsageResource
// ---------------------------------------------------------------------------

func TestInOrMmUsageResource(t *testing.T) {
	r := findResourceRule(t, "InOrMmUsageResource")

	t.Run("mm in layout_width triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_width": "10mm",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "mm") {
			t.Fatalf("expected message to mention mm, got: %s", findings[0].Message)
		}
	})

	t.Run("in in layout_height triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_height": "0.5in",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "in") {
			t.Fatalf("expected message to mention in, got: %s", findings[0].Message)
		}
	})

	t.Run("dp in layout_width is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_width": "16dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("mm in dimens triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Dimensions["button_height"] = "5mm"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("empty index is clean", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// StateListReachableResource
// ---------------------------------------------------------------------------

func TestSpUsageResourceSkipsFixedSizeBox(t *testing.T) {
	r := findResourceRule(t, "SpUsageResource")
	view := func(w, h string) *android.View {
		return &android.View{Type: "TextView", Line: 4, LayoutWidth: w, LayoutHeight: h,
			Attributes: map[string]string{"android:textSize": "12dp", "android:layout_width": w, "android:layout_height": h}}
	}
	if f := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", view("24dp", "24dp"))); len(f) != 0 {
		t.Errorf("fixed dp box should be clean, got %d", len(f))
	}
	if f := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", view("24dp", "wrap_content"))); len(f) != 1 {
		t.Errorf("only one fixed dimension should still report, got %d", len(f))
	}
	if f := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", view("wrap_content", "wrap_content"))); len(f) != 1 {
		t.Errorf("wrap_content text should still report, got %d", len(f))
	}
}

func TestSpUsageResourceSkipsDimenSizedBoxAndFixedParent(t *testing.T) {
	r := findResourceRule(t, "SpUsageResource")
	text := func(w, h string) *android.View {
		return &android.View{Type: "TextView", Line: 4, LayoutWidth: w, LayoutHeight: h,
			Attributes: map[string]string{"android:textSize": "12dp"}}
	}
	withDimens := func(root *android.View) *android.ResourceIndex {
		idx := indexWithLayout("main", "res/layout/main.xml", root)
		idx.Dimensions = map[string]string{"swatch": "40dp", "wide": "match_parent"}
		return idx
	}
	if f := runResourceRule(r, withDimens(text("@dimen/swatch", "@dimen/swatch"))); len(f) != 0 {
		t.Errorf("dimen-sized dp box should be clean, got %d", len(f))
	}
	parent := &android.View{Type: "FrameLayout", Line: 2, LayoutWidth: "40dp", LayoutHeight: "40dp",
		Children: []*android.View{text("wrap_content", "wrap_content")}}
	if f := runResourceRule(r, withDimens(parent)); len(f) != 0 {
		t.Errorf("text inside a fixed dp container should be clean, got %d", len(f))
	}
	flexible := &android.View{Type: "FrameLayout", Line: 2, LayoutWidth: "match_parent", LayoutHeight: "40dp",
		Children: []*android.View{text("wrap_content", "wrap_content")}}
	if f := runResourceRule(r, withDimens(flexible)); len(f) != 1 {
		t.Errorf("a container with only one fixed dimension should still report, got %d", len(f))
	}
}

func TestNegativeMarginResourceSmallTightening(t *testing.T) {
	r := findResourceRule(t, "NegativeMarginResource")
	run := func(orientation, height, margin string) int {
		root := &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{"android:orientation": orientation},
			Children: []*android.View{{Type: "TextView", Line: 3, LayoutHeight: height,
				Attributes: map[string]string{"android:layout_marginTop": margin}}}}
		return len(runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root)))
	}
	if n := run("vertical", "wrap_content", "-4dp"); n != 0 {
		t.Errorf("-4dp tightening should be clean, got %d", n)
	}
	if n := run("vertical", "wrap_content", "-8dp"); n != 1 {
		t.Errorf("-8dp should report, got %d", n)
	}
	if n := run("horizontal", "wrap_content", "-2dp"); n != 1 {
		t.Errorf("horizontal parent should report, got %d", n)
	}
	if n := run("vertical", "48dp", "-2dp"); n != 1 {
		t.Errorf("fixed-height child should report, got %d", n)
	}
}

func TestInOrMmUsageResource_LintWordingAndAnyAttribute(t *testing.T) {
	r := findResourceRule(t, "InOrMmUsageResource")
	root := &android.View{Type: "TextView", Line: 2, Col: 1, Attributes: map[string]string{
		"android:paddingLeft": "1in",
		"android:textSize":    "3.5mm",
		"android:layout_x":    "0mm",
		"android:marginTop":   "-2mm",
		"android:text":        "5mm wide",
	}}
	got := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", root))
	if len(got) != 2 {
		t.Fatalf("expected 2 findings, got %d: %+v", len(got), got)
	}
	want := map[string]bool{
		"Avoid using \"`in`\" as units (it does not work accurately on all devices); use \"`dp`\" instead": true,
		"Avoid using \"`mm`\" as units (it does not work accurately on all devices); use \"`dp`\" instead": true,
	}
	for _, f := range got {
		if !want[f.Message] {
			t.Errorf("unexpected message %q", f.Message)
		}
	}
}
func TestDisableBaselineAlignmentResourceLintShapes(t *testing.T) {
	r := findResourceRule(t, "DisableBaselineAlignmentResource")
	run := func(attrs map[string]string, children ...*android.View) int {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{Type: "LinearLayout", Line: 1, Attributes: attrs, Children: children})
		return len(runResourceRule(r, idx))
	}
	weighted := func(tag string) *android.View {
		return &android.View{Type: tag, Line: 3, Attributes: map[string]string{"android:layout_weight": "1"}}
	}

	if got := run(map[string]string{}, weighted("FrameLayout")); got != 1 {
		t.Errorf("weighted container child should report, got %d", got)
	}
	if got := run(map[string]string{}, weighted("fragment")); got != 1 {
		t.Errorf("weighted fragment child should report, got %d", got)
	}
	if got := run(map[string]string{}, weighted("TextView")); got != 0 {
		t.Errorf("a widget child means baseline alignment matters, got %d", got)
	}
	if got := run(map[string]string{}, weighted("FrameLayout"), &android.View{Type: "TextView", Line: 4, Attributes: map[string]string{}}); got != 0 {
		t.Errorf("one widget child is enough to stay quiet, got %d", got)
	}
	if got := run(map[string]string{}, weighted("RadioGroup")); got != 0 {
		t.Errorf("RadioGroup counts as a widget, got %d", got)
	}
	if got := run(map[string]string{}, weighted("ScrollView")); got != 1 {
		t.Errorf("ScrollView is a layout to Lint's SdkInfo, got %d", got)
	}
	if got := run(map[string]string{}, &android.View{Type: "FrameLayout", Line: 3, Attributes: map[string]string{}}); got != 0 {
		t.Errorf("no weighted child, got %d", got)
	}
	if got := run(map[string]string{"android:orientation": "vertical"}, weighted("FrameLayout")); got != 0 {
		t.Errorf("vertical layouts are exempt, got %d", got)
	}
	for _, value := range []string{"false", "true", "@bool/aligned"} {
		if got := run(map[string]string{"android:baselineAligned": value}, weighted("FrameLayout")); got != 0 {
			t.Errorf("any baselineAligned attribute (%q) stays quiet, got %d", value, got)
		}
	}
}
