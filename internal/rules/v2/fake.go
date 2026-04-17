package v2

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// FakeRule creates a minimal rule for testing. The check function
// receives the context and can emit findings via ctx.Emit().
func FakeRule(id string, opts ...FakeOption) *Rule {
	r := &Rule{
		ID:          id,
		Category:    "test",
		Description: "fake rule for testing",
		Sev:         SeverityWarning,
		Check:       func(*Context) {},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// FakeOption configures a FakeRule.
type FakeOption func(*Rule)

// WithNodeTypes sets the node types the rule dispatches on.
func WithNodeTypes(types ...string) FakeOption {
	return func(r *Rule) { r.NodeTypes = types }
}

// WithNeeds sets the capabilities bitfield.
func WithNeeds(c Capabilities) FakeOption {
	return func(r *Rule) { r.Needs = c }
}

// WithCheck sets the check function.
func WithCheck(fn func(*Context)) FakeOption {
	return func(r *Rule) { r.Check = fn }
}

// WithFix sets the fix level.
func WithFix(level FixLevel) FakeOption {
	return func(r *Rule) { r.Fix = level }
}

// WithConfidence sets the base confidence.
func WithConfidence(c float64) FakeOption {
	return func(r *Rule) { r.Confidence = c }
}

// WithSeverity sets the severity.
func WithSeverity(s Severity) FakeOption {
	return func(r *Rule) { r.Sev = s }
}

// WithOracle sets the oracle filter.
func WithOracle(f *OracleFilter) FakeOption {
	return func(r *Rule) { r.Oracle = f }
}

// FakeContext creates a minimal context for testing with the given file.
func FakeContext(file *scanner.File) *Context {
	return &Context{
		File: file,
	}
}

// FakeContextWithNode creates a context for testing with a specific node index.
func FakeContextWithNode(file *scanner.File, idx uint32) *Context {
	ctx := &Context{
		File: file,
		Idx:  idx,
	}
	if file.FlatTree != nil && int(idx) < len(file.FlatTree.Nodes) {
		node := file.FlatTree.Nodes[idx]
		ctx.Node = &node
	}
	return ctx
}
