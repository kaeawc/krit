package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	rulespkg "github.com/kaeawc/krit/internal/rules"
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

	t.Run("LinearLayout with weight but no orientation triggers", func(t *testing.T) {
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
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("LinearLayout with weight and orientation is clean", func(t *testing.T) {
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
// DisableBaselineAlignmentResource
// ---------------------------------------------------------------------------

func TestDisableBaselineAlignmentResource(t *testing.T) {
	r := findResourceRule(t, "DisableBaselineAlignmentResource")

	t.Run("LinearLayout with weighted children but no baselineAligned triggers", func(t *testing.T) {
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
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("LinearLayout with baselineAligned=false is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "LinearLayout",
			Line: 1,
			Attributes: map[string]string{
				"android:baselineAligned": "false",
			},
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

	t.Run("LinearLayout without weight children is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:       "TextView",
					Line:       3,
					Attributes: map[string]string{},
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
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_marginTop": "-8dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
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
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_marginTop":  "-8dp",
				"android:layout_marginLeft": "-4dp",
			},
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

	t.Run("textSize 11sp triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textSize": "11sp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
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

	t.Run("0dp height in horizontal LinearLayout triggers", func(t *testing.T) {
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
						"android:layout_width":  "wrap_content",
						"android:layout_height": "0dp",
					},
					LayoutWidth:  "wrap_content",
					LayoutHeight: "0dp",
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "layout_height") {
			t.Fatalf("expected message to mention layout_height, got: %s", findings[0].Message)
		}
	})

	t.Run("0dp width in vertical LinearLayout triggers", func(t *testing.T) {
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
						"android:layout_width":  "0dp",
						"android:layout_height": "wrap_content",
					},
					LayoutWidth:  "0dp",
					LayoutHeight: "wrap_content",
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "layout_width") {
			t.Fatalf("expected message to mention layout_width, got: %s", findings[0].Message)
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
