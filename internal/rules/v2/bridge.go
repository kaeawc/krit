package v2

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// V1Rule is the minimal interface that v1 rules.Rule requires.
// Reproduced here to avoid an import cycle with internal/rules.
type V1Rule interface {
	Name() string
	Description() string
	RuleSet() string
	Severity() string
	Check(file *scanner.File) []scanner.Finding
}

// V1FlatDispatchRule is the v1 FlatDispatchRule interface.
type V1FlatDispatchRule interface {
	V1Rule
	NodeTypes() []string
	CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding
}

// V1LineRule is the v1 LineRule interface.
type V1LineRule interface {
	V1Rule
	CheckLines(file *scanner.File) []scanner.Finding
}

// V1CrossFileRule is the v1 CrossFileRule interface.
type V1CrossFileRule interface {
	V1Rule
	CheckCrossFile(index *scanner.CodeIndex) []scanner.Finding
}

// V1ConfidenceProvider is the v1 ConfidenceProvider interface.
type V1ConfidenceProvider interface {
	Confidence() float64
}

// V1FixableRule is the v1 FixableRule interface.
type V1FixableRule interface {
	IsFixable() bool
}

// Verify compile-time interface satisfaction for the v1 compat wrappers.
var (
	_ V1FlatDispatchRule   = (*V1FlatDispatch)(nil)
	_ V1FlatDispatchRule   = (*V1FlatDispatchTypeAware)(nil)
	_ V1LineRule           = (*V1Line)(nil)
	_ V1LineRule           = (*V1LineTypeAware)(nil)
	_ V1CrossFileRule      = (*V1CrossFile)(nil)
	_ V1ConfidenceProvider = (*V1FlatDispatch)(nil)
	_ V1ConfidenceProvider = (*V1FlatDispatchTypeAware)(nil)
	_ V1ConfidenceProvider = (*V1Line)(nil)
	_ V1ConfidenceProvider = (*V1LineTypeAware)(nil)
	_ V1ConfidenceProvider = (*V1CrossFile)(nil)
	_ V1FixableRule        = (*V1FlatDispatch)(nil)
	_ V1FixableRule        = (*V1FlatDispatchTypeAware)(nil)
	_ V1FixableRule        = (*V1Line)(nil)
	_ V1FixableRule        = (*V1LineTypeAware)(nil)
)

// BridgeToV1Rules converts all registered v2 rules into v1-compatible
// wrappers that can be added to the v1 rules.Registry. Call this from
// an init() function or during startup.
func BridgeToV1Rules() []interface{} {
	result := make([]interface{}, len(Registry))
	for i, r := range Registry {
		result[i] = ToV1(r)
	}
	return result
}
