package rules

// Android Resource XML rule: RelativeOverlap, a port of Lint's RelativeOverlapDetector.

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// RelativeOverlapResourceRule mirrors Lint's RelativeOverlap. Inside a RelativeLayout it follows the
// toStartOf/toEndOf chains of the children to find views anchored to the left edge and to the right edge in the
// same vertical bucket. A pair is reported when the text of either chain can grow (a wrap_content view whose text
// comes from a resource, or a button or text view without text) until the two sides meet.
type RelativeOverlapResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *RelativeOverlapResourceRule) Confidence() float64 { return api.ConfidenceMedium }

type relOverlapBucket int

const (
	relOverlapUnset relOverlapBucket = iota
	relOverlapTop
	relOverlapBottom
	relOverlapSkip
)

// relOverlapNode is one child of a RelativeLayout, as Lint's LayoutNode.
type relOverlapNode struct {
	view      *android.View
	processed bool
	bucket    relOverlapBucket
	toLeft    *relOverlapNode
	toRight   *relOverlapNode
	// lastLeft and lastRight mean no other child anchors itself to this side of the view.
	lastLeft  bool
	lastRight bool
}

func newRelOverlapNode(v *android.View) *relOverlapNode {
	return &relOverlapNode{view: v, lastLeft: true, lastRight: true}
}

// relOverlapUniformID turns "@+id/x" into "@id/x" so declarations and references compare equal.
func relOverlapUniformID(id string) string { return strings.Replace(id, "@+", "@", 1) }

func (n *relOverlapNode) attr(name string) string { return n.view.Attributes["android:"+name] }

func (n *relOverlapNode) hasAttr(name string) bool {
	_, ok := n.view.Attributes["android:"+name]
	return ok
}

func (n *relOverlapNode) hasTrue(name string) bool { return n.attr(name) == "true" }

func (n *relOverlapNode) nodeID() string {
	if id := n.attr("id"); id != "" {
		return relOverlapUniformID(id)
	}
	return fmt.Sprintf("%s-%d", n.view.Type, n.view.DOMIndex)
}

func (n *relOverlapNode) textID() string {
	if text := n.attr("text"); text != "" {
		return relOverlapUniformID(text)
	}
	return n.nodeID()
}

func (n *relOverlapNode) invisible() bool {
	v := n.attr("visibility")
	return v == "gone" || v == "invisible"
}

// fixedWidth reports whether the view keeps its width when its text is localized.
func (n *relOverlapNode) fixedWidth() bool {
	if n.attr("layout_width") != "wrap_content" {
		return true
	}
	for _, child := range n.view.Children {
		if !newRelOverlapNode(child).fixedWidth() {
			return false
		}
	}
	if text := n.attr("text"); text != "" {
		return !strings.HasPrefix(text, "@") && !strings.HasPrefix(text, "?")
	}
	tag := n.view.Type
	if strings.Contains(tag, "Image") || strings.Contains(tag, "Progress") || strings.Contains(tag, "Radio") {
		return true
	}
	return !strings.Contains(tag, "Button") && !strings.Contains(tag, "Text")
}

func (n *relOverlapNode) skip() bool {
	return n.bucket == relOverlapSkip || n.view.Type == "include" || n.view.Type == "View"
}

func (n *relOverlapNode) findByAttr(nodes map[string]*relOverlapNode, name string) *relOverlapNode {
	if ref := n.attr(name); ref != "" {
		return nodes[relOverlapUniformID(ref)]
	}
	return nil
}

// processNode assigns the vertical bucket and the horizontal neighbours, resolving a referenced node first.
func (n *relOverlapNode) processNode(nodes map[string]*relOverlapNode) {
	if n.processed {
		return
	}
	n.processed = true
	n.assignBucket(nodes)
	if n.bucket == relOverlapUnset {
		n.bucket = relOverlapTop
	}
	found := n.linkNeighbours(nodes)
	if n.hasTrue("layout_alignParentEnd") || n.hasTrue("layout_alignParentRight") {
		n.lastRight = false
		found = true
	}
	if n.hasTrue("layout_alignParentStart") || n.hasTrue("layout_alignParentLeft") {
		n.lastLeft = false
		found = true
	}
	if !found {
		n.lastLeft = false
	}
}

func (n *relOverlapNode) assignBucket(nodes map[string]*relOverlapNode) {
	switch {
	case n.invisible() || n.hasAttr("layout_alignRight") || n.hasAttr("layout_alignEnd") ||
		n.hasAttr("layout_alignLeft") || n.hasAttr("layout_alignStart"):
		n.bucket = relOverlapSkip
	case n.hasTrue("layout_alignParentTop"):
		n.bucket = relOverlapTop
	case n.hasTrue("layout_alignParentBottom"):
		n.bucket = relOverlapBottom
	case n.hasAttr("layout_above") || n.hasAttr("layout_below"):
		n.bucket = relOverlapSkip
	default:
		for _, name := range []string{"layout_alignTop", "layout_alignBottom", "layout_alignBaseline"} {
			if other := n.findByAttr(nodes, name); other != nil {
				other.processNode(nodes)
				n.bucket = other.bucket
			}
		}
	}
}

// linkNeighbours sets toLeft and toRight and clears the matching last flags. It reports whether either exists.
func (n *relOverlapNode) linkNeighbours(nodes map[string]*relOverlapNode) bool {
	found := false
	n.toLeft = n.findByAttr(nodes, "layout_toStartOf")
	if n.toLeft == nil {
		n.toLeft = n.findByAttr(nodes, "layout_toLeftOf")
	}
	if n.chainReturnsToSelf(n.toLeft, true, len(nodes)) {
		n.toLeft = nil
		n.bucket = relOverlapSkip
	}
	if n.toLeft != nil {
		n.toLeft.lastLeft = false
		n.lastRight = false
		found = true
	}
	n.toRight = n.findByAttr(nodes, "layout_toEndOf")
	if n.toRight == nil {
		n.toRight = n.findByAttr(nodes, "layout_toRightOf")
	}
	if n.chainReturnsToSelf(n.toRight, false, len(nodes)) {
		n.toRight = nil
		n.bucket = relOverlapSkip
	}
	if n.toRight != nil {
		n.toRight.lastRight = false
		n.lastLeft = false
		found = true
	}
	return found
}

// chainReturnsToSelf follows the already linked chain from start looking for n. The step limit stops chains
// that loop without passing through n, which Lint would follow forever.
func (n *relOverlapNode) chainReturnsToSelf(start *relOverlapNode, leftward bool, limit int) bool {
	cur := start
	for steps := 0; cur != nil && steps <= limit; steps++ {
		if cur == n {
			return true
		}
		if leftward {
			cur = cur.toLeft
		} else {
			cur = cur.toRight
		}
	}
	return false
}

// growSet collects the nodes whose text can grow along the chain: toRight for the left side, toLeft for the right.
func (n *relOverlapNode) growSet(leftward bool, limit int) []*relOverlapNode {
	var out []*relOverlapNode
	cur := n
	for steps := 0; cur != nil && steps <= limit; steps++ {
		if !cur.fixedWidth() {
			out = append(out, cur)
		}
		if leftward {
			cur = cur.toRight
		} else {
			cur = cur.toLeft
		}
	}
	// Lint adds the innermost node first, so the chain is reversed.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func relOverlapContains(set []*relOverlapNode, n *relOverlapNode) bool {
	for _, member := range set {
		if member == n {
			return true
		}
	}
	return false
}

// relOverlapFinding is one reported pair.
type relOverlapFinding struct {
	at      *relOverlapNode
	other   *relOverlapNode
	growing []*relOverlapNode
}

func (f relOverlapFinding) message() string {
	texts := make([]string, len(f.growing))
	for i, n := range f.growing {
		texts[i] = n.textID()
	}
	verb := "grows"
	if len(f.growing) > 1 {
		verb = "grow"
	}
	return fmt.Sprintf("`%s` can overlap `%s` if %s %s due to localized text expansion",
		f.at.nodeID(), f.other.nodeID(), strings.Join(texts, ", "), verb)
}

// relOverlapFindings runs Lint's pairing loop over the children of one RelativeLayout.
func relOverlapFindings(layout *android.View) []relOverlapFinding {
	nodes := map[string]*relOverlapNode{}
	var order []string
	for _, child := range layout.Children {
		n := newRelOverlapNode(child)
		id := n.nodeID()
		if _, dup := nodes[id]; !dup {
			order = append(order, id)
		}
		nodes[id] = n
	}
	// Lint iterates a java.util.HashMap, which decides the order in which cyclic references resolve.
	order = javaHashSetOrder(order)
	for _, id := range order {
		nodes[id].processNode(nodes)
	}
	limit := len(nodes)
	var out []relOverlapFinding
	for _, id1 := range order {
		n1 := nodes[id1]
		if !n1.lastLeft || n1.skip() {
			continue
		}
		left := n1.growSet(true, limit)
		for _, id2 := range order {
			n2 := nodes[id2]
			if n2 == n1 || !n2.lastRight || n2.skip() || n1.bucket != n2.bucket {
				continue
			}
			right := n2.growSet(false, limit)
			if len(left) == 0 && len(right) == 0 {
				continue
			}
			for _, n := range left {
				if !relOverlapContains(right, n) {
					right = append(right, n)
				}
			}
			f := relOverlapFinding{at: n1, other: n2, growing: right}
			if !relOverlapContains(right, n1) && relOverlapContains(right, n2) {
				f.at, f.other = n2, n1
			}
			out = append(out, f)
		}
	}
	return out
}

func (r *RelativeOverlapResourceRule) check(ctx *api.Context) {
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"RelativeOverlap"}, func(v *android.View) {
			if v.Type != "RelativeLayout" && v.Type != "android.support.percent.PercentRelativeLayout" {
				return
			}
			for _, f := range relOverlapFindings(v) {
				if resourceToolsIgnoreMatches(f.at.view, "RelativeOverlap") {
					continue
				}
				ctx.Emit(baseFindingAt(layout.FilePath, f.at.view.Line, f.at.view.Col, r.BaseRule, f.message()))
			}
		})
	}
}
