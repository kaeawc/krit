package api

import "sync/atomic"

// DefaultDocsBaseURL is the canonical documentation host that
// DefaultDocsResolver appends rule IDs to when a rule does not set
// DocsURL explicitly. Must end with "/".
const DefaultDocsBaseURL = "https://krit.dev/rules/"

// DocsResolver derives the documentation URL for a rule.
type DocsResolver interface {
	DocsURL(r *Rule) string
}

// DefaultDocsResolver composes "<BaseURL><rule-id>" when Rule.DocsURL is
// empty. A zero value uses DefaultDocsBaseURL; tests may set BaseURL to
// redirect derivation. BaseURL must end with "/" when set.
type DefaultDocsResolver struct {
	BaseURL string
}

// DocsURL implements DocsResolver.
func (d DefaultDocsResolver) DocsURL(r *Rule) string {
	if r == nil {
		return ""
	}
	if r.DocsURL != "" {
		return r.DocsURL
	}
	if r.ID == "" {
		return ""
	}
	base := d.BaseURL
	if base == "" {
		base = DefaultDocsBaseURL
	}
	return base + r.ID
}

var docsResolver atomic.Pointer[DocsResolver]

func init() {
	var d DocsResolver = DefaultDocsResolver{}
	docsResolver.Store(&d)
}

// RuleDocsURL returns the canonical documentation URL for r.
func RuleDocsURL(r *Rule) string {
	return (*docsResolver.Load()).DocsURL(r)
}

// SetDocsResolver swaps the package-level resolver and returns the
// prior value. Passing nil installs the zero-value DefaultDocsResolver.
// Safe for concurrent use.
func SetDocsResolver(d DocsResolver) DocsResolver {
	if d == nil {
		d = DefaultDocsResolver{}
	}
	prev := docsResolver.Swap(&d)
	if prev == nil {
		return DefaultDocsResolver{}
	}
	return *prev
}
