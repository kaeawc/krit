package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestRtlHardcodedResource(t *testing.T) {
	r := findResourceRule(t, "RtlHardcodedResource")

	t.Run("marginLeft triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_marginLeft": "8dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("relative layout and drawable left right attributes trigger", func(t *testing.T) {
		for _, attr := range []string{
			"android:layout_alignParentLeft", "android:layout_toRightOf", "android:layout_alignLeft", "android:drawableLeft",
		} {
			idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
				Type:       "TextView",
				Line:       5,
				Attributes: map[string]string{attr: "true"},
			})
			if findings := runResourceRule(r, idx); len(findings) != 1 {
				t.Fatalf("%s: expected 1 finding, got %d", attr, len(findings))
			}
		}
	})

	t.Run("paddingRight triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:paddingRight": "8dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("marginStart is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_marginStart": "8dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("tools ignore lint issue id is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:paddingLeft": "8dp",
				"tools:ignore":        "RtlHardcoded",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("tools ignore rule id is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:paddingLeft": "8dp",
				"tools:ignore":        "ContentDescription, RtlHardcodedResource",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("unrelated tools ignore still triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:paddingLeft": "8dp",
				"tools:ignore":        "RtlSymmetry",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// LabelForResource
// ---------------------------------------------------------------------------

func TestRtlSymmetryResource(t *testing.T) {
	r := findResourceRule(t, "RtlSymmetryResource")

	t.Run("paddingLeft without paddingRight triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:paddingLeft": "8dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("paddingRight without paddingLeft triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:paddingRight": "8dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("both paddingLeft and paddingRight is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:paddingLeft":  "8dp",
				"android:paddingRight": "8dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("marginLeft without marginRight is clean, as in Lint", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_marginLeft": "16dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("margins are not checked, got %d findings", len(findings))
		}
	})

	t.Run("both marginLeft and marginRight is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_marginLeft":  "16dp",
				"android:layout_marginRight": "16dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no padding or margin is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "TextView",
			Line:       5,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// Registration test — ensure all 30 resource rules are registered
// ---------------------------------------------------------------------------

func TestRtlSuperscriptResource(t *testing.T) {
	r := findResourceRule(t, "RtlSuperscriptResource")

	t.Run("superscript triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textStyle": "superscript",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "RTL") {
			t.Errorf("unexpected message: %s", findings[0].Message)
		}
	})

	t.Run("subscript triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textStyle": "subscript",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("bold is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textStyle": "bold",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no textStyle is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "TextView",
			Line:       5,
			Attributes: map[string]string{},
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
// AdapterViewChildrenResource
// ---------------------------------------------------------------------------

func TestRtlRulesIgnoreElementsWithLogicalAttributes(t *testing.T) {
	hardcoded := findResourceRule(t, "RtlHardcodedResource")
	symmetry := findResourceRule(t, "RtlSymmetryResource")

	both := indexWithLayout("main", "res/layout/main.xml", &android.View{
		Type: "TextView",
		Line: 5,
		Attributes: map[string]string{
			"android:layout_marginLeft":  "8dp",
			"android:layout_marginStart": "8dp",
		},
	})
	if findings := runResourceRule(hardcoded, both); len(findings) != 1 || !strings.HasPrefix(findings[0].Message, "Redundant attribute `layout_marginLeft`; already defining `layout_marginStart`") {
		t.Errorf("RtlHardcodedResource: marginLeft next to marginStart is redundant, got %v", findingMessages(findings))
	}
	if findings := runResourceRule(symmetry, both); len(findings) != 0 {
		t.Errorf("RtlSymmetryResource: marginLeft with marginStart should be clean, got %d", len(findings))
	}

	ignored := indexWithLayout("main", "res/layout/main.xml", &android.View{
		Type: "TextView",
		Line: 5,
		Attributes: map[string]string{
			"android:paddingLeft": "8dp",
			"tools:ignore":        "RtlSymmetry",
		},
	})
	if findings := runResourceRule(symmetry, ignored); len(findings) != 0 {
		t.Errorf("RtlSymmetryResource: tools:ignore=RtlSymmetry should suppress, got %d", len(findings))
	}
}

func TestRtlSymmetryResourceFollowsLintPaddingPairs(t *testing.T) {
	symmetry := findResourceRule(t, "RtlSymmetryResource")
	run := func(attrs map[string]string) []scanner.Finding {
		return runResourceRule(symmetry, indexWithLayout("main", "res/layout/main.xml",
			&android.View{Type: "TextView", Line: 5, Attributes: attrs}))
	}

	if f := run(map[string]string{"android:paddingLeft": "8dp", "android:paddingEnd": "8dp"}); len(f) != 0 {
		t.Errorf("paddingLeft with paddingEnd is symmetric, got %d", len(f))
	}
	if f := run(map[string]string{"android:paddingStart": "8dp", "android:paddingRight": "8dp"}); len(f) != 0 {
		t.Errorf("paddingStart with paddingRight is symmetric, got %d", len(f))
	}
	f := run(map[string]string{"android:paddingStart": "8dp"})
	if len(f) != 1 || f[0].Message != "When you define `paddingStart` you should probably also define `paddingEnd` for right-to-left symmetry" {
		t.Errorf("lone paddingStart should report Lint's message, got %+v", f)
	}
	if f := run(map[string]string{"android:paddingLeft": "8dp", "tools:ignore": "RtlHardcoded"}); len(f) != 1 {
		t.Errorf("tools:ignore=RtlHardcoded must not suppress RtlSymmetry, got %d", len(f))
	}
	if f := run(map[string]string{"android:paddingLeft": "8dp", "tools:ignore": "RtlSymmetry"}); len(f) != 0 {
		t.Errorf("tools:ignore=RtlSymmetry should suppress, got %d", len(f))
	}
}

func TestRtlHardcodedResourceReportsEveryElement(t *testing.T) {
	hardcoded := findResourceRule(t, "RtlHardcodedResource")
	child := func(line int) *android.View {
		return &android.View{Type: "TextView", Line: line, Attributes: map[string]string{"android:paddingLeft": "4dp"}}
	}
	root := &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{"android:paddingRight": "4dp"},
		Children: []*android.View{child(3), {Type: "FrameLayout", Line: 5, Attributes: map[string]string{}, Children: []*android.View{child(6)}}, child(8)}}
	findings := runResourceRule(hardcoded, indexWithLayout("main", "res/layout/main.xml", root))
	if len(findings) != 4 {
		t.Fatalf("expected one finding per offending element (4), got %d", len(findings))
	}
}

func TestRtlHardcodedResourceFollowsLintMessagesAndSkips(t *testing.T) {
	hardcoded := findResourceRule(t, "RtlHardcodedResource")
	run := func(attrs map[string]string) []scanner.Finding {
		return runResourceRule(hardcoded, indexWithLayout("main", "res/layout/main.xml",
			&android.View{Type: "TextView", Line: 5, Attributes: attrs}))
	}
	if f := run(map[string]string{"android:paddingLeft": "8dp", "android:paddingRight": "8dp"}); len(f) != 0 {
		t.Errorf("equal left and right padding is symmetric, got %v", findingMessages(f))
	}
	f := run(map[string]string{"android:paddingLeft": "8dp", "android:paddingRight": "4dp"})
	if len(f) != 2 || f[0].Message != "Consider replacing `android:paddingLeft` with `android:paddingStart=\"8dp\"` to better support right-to-left layouts" {
		t.Errorf("one finding per attribute with Lint's wording, got %v", findingMessages(f))
	}
	f = run(map[string]string{"android:layout_gravity": "left|center"})
	if len(f) != 1 || f[0].Message != "Use \"`start`\" instead of \"`left`\" to ensure correct behavior in right-to-left locales" {
		t.Errorf("gravity left should suggest start, got %v", findingMessages(f))
	}
	if f := run(map[string]string{"android:gravity": "center"}); len(f) != 0 {
		t.Errorf("gravity without left or right is clean, got %v", findingMessages(f))
	}
}
