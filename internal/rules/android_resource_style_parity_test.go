package rules_test

// Tests covering AOSP PxUsageDetector parity additions:
//   - style <item> walks for PxUsage / InOrMmUsage / SpUsage / SmallSp
//   - 0px / 1px / 0mm / 0in exemptions
//   - @dimen/... resolution on android:textSize

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

// ---------------------------------------------------------------------------
// 0px / 1px and 0mm / 0in exemptions
// ---------------------------------------------------------------------------

func TestPxUsageResourceExemptions(t *testing.T) {
	r := findResourceRule(t, "PxUsageResource")

	for _, val := range []string{"0px", "0.0px", "1px"} {
		t.Run("layout_width "+val+" is clean", func(t *testing.T) {
			idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
				Type: "View",
				Line: 5,
				Attributes: map[string]string{
					"android:layout_height": val,
				},
			})
			findings := runResourceRule(r, idx)
			if len(findings) != 0 {
				t.Fatalf("expected 0 findings for %q, got %d", val, len(findings))
			}
		})
	}

	t.Run("layout_height 2px still triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "View",
			Line: 7,
			Attributes: map[string]string{
				"android:layout_height": "2px",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("dimen value 1px is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Dimensions["hairline"] = "1px"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d (%v)", len(findings), findings)
		}
	})
}

func TestInOrMmUsageResourceExemptions(t *testing.T) {
	r := findResourceRule(t, "InOrMmUsageResource")

	for _, val := range []string{"0mm", "0in", "0.0mm"} {
		t.Run("layout_width "+val+" is clean", func(t *testing.T) {
			idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
				Type: "View",
				Line: 5,
				Attributes: map[string]string{
					"android:layout_width": val,
				},
			})
			findings := runResourceRule(r, idx)
			if len(findings) != 0 {
				t.Fatalf("expected 0 findings for %q, got %d", val, len(findings))
			}
		})
	}

	t.Run("non-zero mm still triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "View",
			Line: 9,
			Attributes: map[string]string{
				"android:layout_width": "5mm",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// Style <item> walks
// ---------------------------------------------------------------------------

func styleIndex(path string, style *android.Style) *android.ResourceIndex {
	idx := emptyIndex()
	idx.Styles[style.Name] = style
	if style.FilePath == "" {
		style.FilePath = path
	}
	return idx
}

func TestPxUsageResourceStyleItem(t *testing.T) {
	r := findResourceRule(t, "PxUsageResource")

	t.Run("style item with px triggers with item line", func(t *testing.T) {
		idx := styleIndex("res/values/styles.xml", &android.Style{
			Name: "MyTheme",
			Line: 3,
			Items: map[string]string{
				"android:layout_width": "24px",
			},
			ItemLines: map[string]int{
				"android:layout_width": 4,
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 4 {
			t.Fatalf("expected finding line 4 (item line), got %d", findings[0].Line)
		}
		if !strings.Contains(findings[0].Message, "MyTheme") {
			t.Fatalf("finding message missing style name: %q", findings[0].Message)
		}
	})

	t.Run("style item 1px is clean", func(t *testing.T) {
		idx := styleIndex("res/values/styles.xml", &android.Style{
			Name: "MyTheme",
			Items: map[string]string{
				"android:layout_height": "1px",
			},
			ItemLines: map[string]int{
				"android:layout_height": 4,
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d (%v)", len(findings), findings)
		}
	})
}

func TestInOrMmUsageResourceStyleItem(t *testing.T) {
	r := findResourceRule(t, "InOrMmUsageResource")

	t.Run("style item with mm triggers", func(t *testing.T) {
		idx := styleIndex("res/values/styles.xml", &android.Style{
			Name: "MyTheme",
			Items: map[string]string{
				"android:padding": "3mm",
			},
			ItemLines: map[string]int{
				"android:padding": 7,
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 7 {
			t.Fatalf("expected finding line 7, got %d", findings[0].Line)
		}
	})
}

func TestSpUsageResourceStyleItem(t *testing.T) {
	r := findResourceRule(t, "SpUsageResource")

	t.Run("style item textSize in dp triggers", func(t *testing.T) {
		idx := styleIndex("res/values/styles.xml", &android.Style{
			Name: "TextAppearance.Title",
			Items: map[string]string{
				"android:textSize": "14dp",
			},
			ItemLines: map[string]int{
				"android:textSize": 5,
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 5 {
			t.Fatalf("expected finding line 5, got %d", findings[0].Line)
		}
	})

	t.Run("style item textSize in sp is clean", func(t *testing.T) {
		idx := styleIndex("res/values/styles.xml", &android.Style{
			Name: "TextAppearance.Title",
			Items: map[string]string{
				"android:textSize": "14sp",
			},
			ItemLines: map[string]int{
				"android:textSize": 5,
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d (%v)", len(findings), findings)
		}
	})

	t.Run("style item non-textSize attr in dp is clean", func(t *testing.T) {
		// SpUsage is only about textSize. layout_margin in dp is fine.
		idx := styleIndex("res/values/styles.xml", &android.Style{
			Name: "MyTheme",
			Items: map[string]string{
				"android:layout_margin": "16dp",
			},
			ItemLines: map[string]int{
				"android:layout_margin": 5,
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d (%v)", len(findings), findings)
		}
	})
}

func TestSmallSpResourceStyleItem(t *testing.T) {
	r := findResourceRule(t, "SmallSpResource")

	t.Run("style item textSize 8sp triggers", func(t *testing.T) {
		idx := styleIndex("res/values/styles.xml", &android.Style{
			Name: "TextAppearance.Caption",
			Items: map[string]string{
				"android:textSize": "8sp",
			},
			ItemLines: map[string]int{
				"android:textSize": 6,
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 6 {
			t.Fatalf("expected finding line 6, got %d", findings[0].Line)
		}
	})

	t.Run("style item textSize 12sp is clean", func(t *testing.T) {
		idx := styleIndex("res/values/styles.xml", &android.Style{
			Name: "TextAppearance.Body",
			Items: map[string]string{
				"android:textSize": "12sp",
			},
			ItemLines: map[string]int{
				"android:textSize": 6,
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d (%v)", len(findings), findings)
		}
	})
}

// ---------------------------------------------------------------------------
// @dimen/... resolution on android:textSize
// ---------------------------------------------------------------------------

func TestSpUsageResourceDimenReference(t *testing.T) {
	r := findResourceRule(t, "SpUsageResource")

	t.Run("textSize=@dimen/x where x is dp triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 8,
			Attributes: map[string]string{
				"android:textSize": "@dimen/title_size",
			},
		})
		idx.Dimensions["title_size"] = "14dp"
		idx.DimensionsLocation["title_size"] = android.StringLocation{
			FilePath: "res/values/dimens.xml",
			Line:     12,
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 8 {
			t.Fatalf("expected finding at usage line 8, got %d", findings[0].Line)
		}
		if !strings.Contains(findings[0].Message, "dimens.xml") {
			t.Fatalf("finding should mention resolved dimen origin; got %q", findings[0].Message)
		}
	})

	t.Run("textSize=@dimen/x where x is sp is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 8,
			Attributes: map[string]string{
				"android:textSize": "@dimen/title_size",
			},
		})
		idx.Dimensions["title_size"] = "14sp"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d (%v)", len(findings), findings)
		}
	})

	t.Run("textSize=@dimen/x where x is unknown is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 8,
			Attributes: map[string]string{
				"android:textSize": "@dimen/missing",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d (%v)", len(findings), findings)
		}
	})
}

func TestSmallSpResourceDimenReference(t *testing.T) {
	r := findResourceRule(t, "SmallSpResource")

	t.Run("textSize=@dimen/x where x is 8sp triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 8,
			Attributes: map[string]string{
				"android:textSize": "@dimen/tiny",
			},
		})
		idx.Dimensions["tiny"] = "8sp"
		idx.DimensionsLocation["tiny"] = android.StringLocation{
			FilePath: "res/values/dimens.xml",
			Line:     3,
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 8 {
			t.Fatalf("expected finding at usage line 8, got %d", findings[0].Line)
		}
	})

	t.Run("textSize=@dimen/x where x is 16sp is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 8,
			Attributes: map[string]string{
				"android:textSize": "@dimen/body",
			},
		})
		idx.Dimensions["body"] = "16sp"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d (%v)", len(findings), findings)
		}
	})
}

// ---------------------------------------------------------------------------
// Dimens findings carry the dimens.xml location now
// ---------------------------------------------------------------------------

func TestPxUsageResourceDimensLocation(t *testing.T) {
	r := findResourceRule(t, "PxUsageResource")

	t.Run("dimen finding uses recorded file and line", func(t *testing.T) {
		idx := emptyIndex()
		idx.Dimensions["margin_large"] = "24px"
		idx.DimensionsLocation["margin_large"] = android.StringLocation{
			FilePath: "app/src/main/res/values/dimens.xml",
			Line:     17,
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].File != "app/src/main/res/values/dimens.xml" {
			t.Fatalf("expected finding in real dimens path, got %q", findings[0].File)
		}
		if findings[0].Line != 17 {
			t.Fatalf("expected finding line 17, got %d", findings[0].Line)
		}
	})
}
