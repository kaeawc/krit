package rules

import (
	"fmt"
	"sort"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// LanguageSupportFilter narrows a registry by per-language LanguageSupport
// status. An empty Status list matches any status (so a filter is meaningful
// as long as Language is set). Negate flips the membership check.
type LanguageSupportFilter struct {
	Language string
	Status   []api.LanguageSupportStatus
	Negate   bool
}

// IsZero reports whether the filter would not narrow the registry.
func (f LanguageSupportFilter) IsZero() bool {
	return f.Language == "" && len(f.Status) == 0 && !f.Negate
}

// Validate normalizes f and reports any unknown status values. An empty
// Language is allowed only when Status is also empty (a no-op filter).
func (f LanguageSupportFilter) Validate() error {
	if f.Language == "" && (len(f.Status) > 0 || f.Negate) {
		return fmt.Errorf("language support filter: language is required when status or negate is set")
	}
	for _, s := range f.Status {
		if !s.Valid() {
			return fmt.Errorf("language support filter: unknown status %q", s)
		}
	}
	return nil
}

// Matches reports whether the rule's resolved support for f.Language passes
// the filter. A nil rule never matches.
func (f LanguageSupportFilter) Matches(r *api.Rule) bool {
	if r == nil {
		return false
	}
	if f.IsZero() {
		return true
	}
	support, ok := LanguageSupportForRule(r, f.Language)
	if !ok {
		return f.Negate
	}
	if len(f.Status) == 0 {
		return !f.Negate
	}
	for _, s := range f.Status {
		if s == support.Status {
			return !f.Negate
		}
	}
	return f.Negate
}

// LanguageSupportForRule returns the per-language support entry for the
// given language key, falling back to ruleset defaults when no per-rule
// override exists. Currently only "java" is wired to a source-of-truth
// matrix; other languages return (zero, false).
func LanguageSupportForRule(r *api.Rule, language string) (api.LanguageSupport, bool) {
	if r == nil || language == "" {
		return api.LanguageSupport{}, false
	}
	switch language {
	case JavaLanguageSupportKey:
		return JavaSupportForRule(r)
	default:
		return api.LanguageSupport{}, false
	}
}

// FilterRegistry returns rules from the registry that match f, in registry
// order. Returns a fresh slice; the registry itself is not modified.
func FilterRegistry(registry []*api.Rule, f LanguageSupportFilter) []*api.Rule {
	if f.IsZero() {
		out := make([]*api.Rule, 0, len(registry))
		for _, r := range registry {
			if r != nil {
				out = append(out, r)
			}
		}
		return out
	}
	out := make([]*api.Rule, 0, len(registry))
	for _, r := range registry {
		if f.Matches(r) {
			out = append(out, r)
		}
	}
	return out
}

// ParseStatusFilter parses a status filter expression of the form
// "supported", "partial,pending", or "!supported". The leading "!"
// applies to the whole expression. Spaces around commas are tolerated.
// Empty input returns (nil, false, nil).
func ParseStatusFilter(expr string) ([]api.LanguageSupportStatus, bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, false, nil
	}
	negate := false
	if strings.HasPrefix(expr, "!") {
		negate = true
		expr = strings.TrimSpace(strings.TrimPrefix(expr, "!"))
	}
	if expr == "" {
		return nil, negate, fmt.Errorf("status filter: expected at least one status after '!'")
	}
	raw := strings.Split(expr, ",")
	statuses := make([]api.LanguageSupportStatus, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		status := api.LanguageSupportStatus(s)
		if !status.Valid() {
			return nil, negate, fmt.Errorf("status filter: unknown status %q", s)
		}
		statuses = append(statuses, status)
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i] < statuses[j] })
	return statuses, negate, nil
}
