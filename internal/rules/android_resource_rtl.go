package rules

// Android Resource XML rules: RTL (right-to-left) rules.

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

// ---------------------------------------------------------------------------
// RtlHardcodedResource
// ---------------------------------------------------------------------------

// RtlHardcodedResourceRule detects layout_marginLeft/Right and paddingLeft/Right
// instead of their Start/End equivalents, which breaks RTL support.
type RtlHardcodedResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

var rtlReplacements = map[string]string{
	"android:layout_marginLeft":  "android:layout_marginStart",
	"android:layout_marginRight": "android:layout_marginEnd",
	"android:paddingLeft":        "android:paddingStart",
	"android:paddingRight":       "android:paddingEnd",
}

// Confidence reports a tier-2 (medium) base confidence. Android RTL resource rule. Detection flags start/end vs left/right
// attribute usage and bidi markers via attribute presence checks.
// Classified per roadmap/17.
func (r *RtlHardcodedResourceRule) Confidence() float64 { return 0.75 }

func (r *RtlHardcodedResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	// Iterate attributes in a stable order so a view with multiple
	// hardcoded Left/Right attributes produces deterministic messages.
	stableAttrs := []string{
		"android:layout_marginLeft", "android:layout_marginRight",
		"android:paddingLeft", "android:paddingRight",
	}
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			var replacements []struct{ old, new string }
			for _, oldAttr := range stableAttrs {
				if v.Attributes[oldAttr] != "" {
					replacements = append(replacements, struct{ old, new string }{oldAttr, rtlReplacements[oldAttr]})
				}
			}
			if len(replacements) == 0 {
				return
			}
			// Combine all hardcoded RTL attribute replacements for this
			// view into a single finding. Emitting one finding per
			// attribute at the same (file, line) collided on the finding
			// key downstream.
			if len(replacements) == 1 {
				findings = append(findings, resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("Use `%s` instead of `%s` for RTL support in `%s`.",
						replacements[0].new, replacements[0].old, v.Type)))
				return
			}
			var parts []string
			for _, rep := range replacements {
				parts = append(parts, fmt.Sprintf("`%s` -> `%s`", rep.old, rep.new))
			}
			findings = append(findings, resourceFinding(layout.FilePath, v.Line, r.BaseRule,
				fmt.Sprintf("Use Start/End instead of Left/Right in `%s` for RTL support: %s.",
					v.Type, strings.Join(parts, ", "))))
		})
	}
	return findings
}

// ---------------------------------------------------------------------------
// RtlSymmetryResource
// ---------------------------------------------------------------------------

// RtlSymmetryResourceRule detects asymmetric padding or margin. If a view
// specifies paddingLeft but not paddingRight (or marginLeft but not
// marginRight), the layout will look unbalanced in RTL locales.
type RtlSymmetryResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

var symmetryPairs = [][2]string{
	{"android:paddingLeft", "android:paddingRight"},
	{"android:layout_marginLeft", "android:layout_marginRight"},
}

// hasSingleEdgeConstraint returns true if the view is anchored to only one
// horizontal edge via ConstraintLayout constraints (e.g. constrained to the
// right edge with no matching left constraint). Such views have intentionally
// asymmetric margins because they're floated to one side.
func hasSingleEdgeConstraint(v *android.View) bool {
	leftAnchors := []string{
		"app:layout_constraintLeft_toLeftOf", "app:layout_constraintLeft_toRightOf",
		"app:layout_constraintStart_toStartOf", "app:layout_constraintStart_toEndOf",
	}
	rightAnchors := []string{
		"app:layout_constraintRight_toRightOf", "app:layout_constraintRight_toLeftOf",
		"app:layout_constraintEnd_toEndOf", "app:layout_constraintEnd_toStartOf",
	}
	hasLeftAnchor := false
	hasRightAnchor := false
	for _, a := range leftAnchors {
		if v.Attributes[a] != "" {
			hasLeftAnchor = true
			break
		}
	}
	for _, a := range rightAnchors {
		if v.Attributes[a] != "" {
			hasRightAnchor = true
			break
		}
	}
	return (hasLeftAnchor && !hasRightAnchor) || (hasRightAnchor && !hasLeftAnchor)
}

// Confidence reports a tier-2 (medium) base confidence. Android RTL resource rule. Detection flags start/end vs left/right
// attribute usage and bidi markers via attribute presence checks.
// Classified per roadmap/17.
func (r *RtlSymmetryResourceRule) Confidence() float64 { return 0.75 }

func (r *RtlSymmetryResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			// Skip views anchored to only one horizontal edge — asymmetry is intentional.
			if hasSingleEdgeConstraint(v) {
				return
			}
			for _, pair := range symmetryPairs {
				hasLeft := v.Attributes[pair[0]] != ""
				hasRight := v.Attributes[pair[1]] != ""
				if hasLeft && !hasRight {
					findings = append(findings, resourceFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("`%s` has `%s` but not `%s`. Add the missing attribute for symmetric spacing.",
							v.Type, pair[0], pair[1])))
				} else if hasRight && !hasLeft {
					findings = append(findings, resourceFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("`%s` has `%s` but not `%s`. Add the missing attribute for symmetric spacing.",
							v.Type, pair[1], pair[0])))
				}
			}
		})
	}
	return findings
}

// ---------------------------------------------------------------------------
// RtlSuperscriptResource
// ---------------------------------------------------------------------------

// RtlSuperscriptResourceRule detects superscript/subscript text styling that
// may break in RTL locales.
type RtlSuperscriptResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android RTL resource rule. Detection flags start/end vs left/right
// attribute usage and bidi markers via attribute presence checks.
// Classified per roadmap/17.
func (r *RtlSuperscriptResourceRule) Confidence() float64 { return 0.75 }

func (r *RtlSuperscriptResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			textStyle := v.Attributes["android:textStyle"]
			if textStyle == "" {
				return
			}
			lower := strings.ToLower(textStyle)
			if strings.Contains(lower, "superscript") || strings.Contains(lower, "subscript") {
				findings = append(findings, resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`%s` uses textStyle `%s` which may render incorrectly in RTL locales.", v.Type, textStyle)))
			}
		})
	}
	return findings
}

// ---------------------------------------------------------------------------
// RelativeOverlapResource
// ---------------------------------------------------------------------------

// RelativeOverlapResourceRule detects views inside a RelativeLayout that may
// overlap because multiple children use alignParentLeft (or alignParentStart)
// without different vertical positioning constraints.
type RelativeOverlapResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

var verticalConstraintAttrs = []string{
	"android:layout_below", "android:layout_above",
	"android:layout_alignTop", "android:layout_alignBottom",
	"android:layout_alignParentTop", "android:layout_alignParentBottom",
	"android:layout_centerVertical",
}

func verticalConstraintKey(v *android.View) string {
	for _, attr := range verticalConstraintAttrs {
		if val := v.Attributes[attr]; val != "" {
			return attr + "=" + val
		}
	}
	return ""
}

// Confidence reports a tier-2 (medium) base confidence. Android RTL resource rule. Detection flags start/end vs left/right
// attribute usage and bidi markers via attribute presence checks.
// Classified per roadmap/17.
func (r *RelativeOverlapResourceRule) Confidence() float64 { return 0.75 }

func (r *RelativeOverlapResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "RelativeLayout" {
				return
			}
			// Collect children that align to the left
			type leftChild struct {
				view    *android.View
				vertKey string
			}
			var leftAligned []leftChild
			for _, child := range v.Children {
				if child.Attributes["android:layout_alignParentLeft"] == "true" ||
					child.Attributes["android:layout_alignParentStart"] == "true" {
					leftAligned = append(leftAligned, leftChild{
						view:    child,
						vertKey: verticalConstraintKey(child),
					})
				}
			}
			// Check for pairs with same vertical constraint key (including both empty)
			for i := 0; i < len(leftAligned); i++ {
				for j := i + 1; j < len(leftAligned); j++ {
					if leftAligned[i].vertKey == leftAligned[j].vertKey {
						findings = append(findings, resourceFinding(layout.FilePath, leftAligned[j].view.Line, r.BaseRule,
							fmt.Sprintf("`%s` (line %d) and `%s` (line %d) in `RelativeLayout` both use left/start alignment "+
								"without different vertical constraints and may overlap.",
								leftAligned[i].view.Type, leftAligned[i].view.Line,
								leftAligned[j].view.Type, leftAligned[j].view.Line)))
					}
				}
			}
		})
	}
	return findings
}

// ---------------------------------------------------------------------------
// NotSiblingResource
// ---------------------------------------------------------------------------

// NotSiblingResourceRule detects RelativeLayout constraints that reference IDs
// not belonging to a sibling view. When layout_below, layout_above, etc.
// reference an @id/ that is not a direct child of the same RelativeLayout,
// the constraint is invalid.
type NotSiblingResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

var relativeConstraintAttrs = []string{
	"android:layout_below",
	"android:layout_above",
	"android:layout_toLeftOf",
	"android:layout_toRightOf",
	"android:layout_toStartOf",
	"android:layout_toEndOf",
	"android:layout_alignBaseline",
	"android:layout_alignLeft",
	"android:layout_alignRight",
	"android:layout_alignTop",
	"android:layout_alignBottom",
	"android:layout_alignStart",
	"android:layout_alignEnd",
}

// Confidence reports a tier-2 (medium) base confidence. Android RTL resource rule. Detection flags start/end vs left/right
// attribute usage and bidi markers via attribute presence checks.
// Classified per roadmap/17.
func (r *NotSiblingResourceRule) Confidence() float64 { return 0.75 }

func (r *NotSiblingResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		checkNotSibling(layout.RootView, layout.FilePath, r.BaseRule, &findings)
	}
	return findings
}

func checkNotSibling(v *android.View, filePath string, base BaseRule, findings *[]scanner.Finding) {
	if v == nil {
		return
	}
	if v.Type == "RelativeLayout" {
		// Collect child IDs (strip @+id/ and @id/ prefixes)
		siblingIDs := make(map[string]bool)
		for _, child := range v.Children {
			if child.ID != "" {
				id := child.ID
				id = strings.TrimPrefix(id, "@+id/")
				id = strings.TrimPrefix(id, "@id/")
				siblingIDs[id] = true
			}
		}
		// Check constraint attrs on each child
		for _, child := range v.Children {
			for _, attr := range relativeConstraintAttrs {
				ref := child.Attributes[attr]
				if ref == "" {
					continue
				}
				refID := ref
				refID = strings.TrimPrefix(refID, "@+id/")
				refID = strings.TrimPrefix(refID, "@id/")
				if refID == "" {
					continue
				}
				if !siblingIDs[refID] {
					*findings = append(*findings, resourceFinding(filePath, child.Line, base,
						fmt.Sprintf("`%s=\"%s\"` references `%s` which is not a sibling in this RelativeLayout.",
							attr, ref, refID)))
				}
			}
		}
	}
	// Recurse into children to find nested RelativeLayouts
	for _, child := range v.Children {
		checkNotSibling(child, filePath, base, findings)
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
