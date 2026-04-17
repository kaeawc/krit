package rules

import (
	"sync"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// V2Index stores v2.Rule references grouped by capability,
// enabling gradual migration of dispatch logic to v2.
type V2Index struct {
	All         []*v2.Rule            // all rules in v2 form
	NodeRules   []*v2.Rule            // per-file node dispatch rules
	LineRules   []*v2.Rule            // per-file line scan rules
	CrossFile   []*v2.Rule            // cross-file rules
	ModuleAware []*v2.Rule            // module-aware rules
	Legacy      []*v2.Rule            // legacy Check() rules
	ByID        map[string]*v2.Rule   // lookup by rule ID
}

// BuildV2Index converts a slice of v1 rules into a V2Index, wrapping each
// rule via WrapAsV2 and classifying it by its v1 interface type.
func BuildV2Index(activeRules []Rule) *V2Index {
	idx := &V2Index{
		All:  make([]*v2.Rule, 0, len(activeRules)),
		ByID: make(map[string]*v2.Rule, len(activeRules)),
	}

	for _, r := range activeRules {
		wrapped := WrapAsV2(r)
		idx.All = append(idx.All, wrapped)
		idx.ByID[wrapped.ID] = wrapped

		// Classify by the v2.Rule's declared Needs bitfield plus a
		// structural check for the CheckFlatNode / CollectFlatNode
		// method set on the original rule. This preserves the v1
		// distinction between "node-dispatch with nil NodeTypes"
		// (receives every node) and "legacy with only Check()".
		switch {
		case wrapped.Needs.Has(v2.NeedsCrossFile):
			idx.CrossFile = append(idx.CrossFile, wrapped)
		case wrapped.Needs.Has(v2.NeedsModuleIndex):
			idx.ModuleAware = append(idx.ModuleAware, wrapped)
		case wrapped.Needs.Has(v2.NeedsLinePass):
			idx.LineRules = append(idx.LineRules, wrapped)
		default:
			if isNodeDispatchRule(r) {
				idx.NodeRules = append(idx.NodeRules, wrapped)
			} else {
				idx.Legacy = append(idx.Legacy, wrapped)
			}
		}
	}

	return idx
}

// isNodeDispatchRule reports whether r's method set puts it in the
// node-dispatch family (FlatDispatch or Aggregate) as opposed to being
// a legacy Check()-only rule.
func isNodeDispatchRule(r Rule) bool {
	if _, ok := r.(interface {
		NodeTypes() []string
		CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding
	}); ok {
		return true
	}
	if _, ok := r.(interface {
		AggregateNodeTypes() []string
		CollectFlatNode(idx uint32, file *scanner.File)
		Finalize(file *scanner.File) []scanner.Finding
	}); ok {
		return true
	}
	return false
}

// v2IndexOnce guards lazy initialization of the cached V2Index.
var v2IndexField struct {
	// embedded in Dispatcher via the method below
}

// v2Index is the cached index stored on a Dispatcher.
// We use a separate struct to avoid modifying dispatch.go.
var (
	v2CacheMu sync.Mutex
	v2Cache    = map[*Dispatcher]*V2Index{}
)

// V2Rules lazily builds and caches the V2Index for this dispatcher.
// The index is derived from the same active rules that were used to
// construct the dispatcher.
func (d *Dispatcher) V2Rules() *V2Index {
	v2CacheMu.Lock()
	defer v2CacheMu.Unlock()

	if idx, ok := v2Cache[d]; ok {
		return idx
	}

	idx := BuildV2Index(d.activeRules)
	v2Cache[d] = idx
	return idx
}
