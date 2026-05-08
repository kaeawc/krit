package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
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

	t.Run("marginLeft without marginRight triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:layout_marginLeft": "16dp",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
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
// AlwaysShowActionResource
// ---------------------------------------------------------------------------

func TestRelativeOverlapResource(t *testing.T) {
	r := findResourceRule(t, "RelativeOverlapResource")

	t.Run("two children with alignParentLeft and no vertical constraint triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "RelativeLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_alignParentLeft": "true",
					},
				},
				{
					Type: "TextView",
					Line: 7,
					Attributes: map[string]string{
						"android:layout_alignParentLeft": "true",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("two children with alignParentLeft but different vertical constraints is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "RelativeLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_alignParentLeft": "true",
						"android:layout_alignParentTop":  "true",
					},
				},
				{
					Type: "TextView",
					Line: 7,
					Attributes: map[string]string{
						"android:layout_alignParentLeft":   "true",
						"android:layout_alignParentBottom": "true",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-RelativeLayout is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "TextView",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_alignParentLeft": "true",
					},
				},
				{
					Type: "TextView",
					Line: 7,
					Attributes: map[string]string{
						"android:layout_alignParentLeft": "true",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("alignParentStart overlap also triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "RelativeLayout",
			Line:       1,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type: "Button",
					Line: 3,
					Attributes: map[string]string{
						"android:layout_alignParentStart": "true",
					},
				},
				{
					Type: "Button",
					Line: 7,
					Attributes: map[string]string{
						"android:layout_alignParentStart": "true",
					},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
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
