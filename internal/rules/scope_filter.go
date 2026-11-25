package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
)

// Scope group names accepted by ParseScopeSet alongside the individual
// dispatcher scope labels.
const (
	// ScopeGroupFile names the rules whose findings for a file depend only on
	// that file (and on type information): the per-file AST, line and
	// aggregate passes.
	ScopeGroupFile = "file"
	// ScopeGroupProject names every other rule: cross-file and module-index
	// rules, rules over all parsed files, and Android manifest, resource,
	// icon and Gradle rules. Their findings can change when another file or
	// module changes.
	ScopeGroupProject = "project"
)

// fileScopes are the dispatcher scopes in ScopeGroupFile.
var fileScopes = []api.Scope{
	api.ScopePerFileNode,
	api.ScopePerFileAllNodes,
	api.ScopeLinePass,
	api.ScopeAggregate,
}

// projectScopes are the dispatcher scopes in ScopeGroupProject.
var projectScopes = []api.Scope{
	api.ScopeCrossFile,
	api.ScopeModuleIndex,
	api.ScopeParsedFiles,
	api.ScopeManifest,
	api.ScopeResource,
	api.ScopeResourceSource,
	api.ScopeIcons,
	api.ScopeGradle,
	api.ScopeManifestSource,
}

// ScopeSet is a set of dispatcher scopes. FilterByScope treats a nil set as
// "no filter".
type ScopeSet map[api.Scope]bool

// ScopeGroup returns the scope group of r: ScopeGroupFile or
// ScopeGroupProject.
func ScopeGroup(r *api.Rule) string {
	scope := RuleScope(r)
	for _, f := range fileScopes {
		if scope == f {
			return ScopeGroupFile
		}
	}
	return ScopeGroupProject
}

// ScopeTokens lists every token ParseScopeSet accepts, groups first.
func ScopeTokens() []string {
	labels := make([]string, 0, len(fileScopes)+len(projectScopes))
	for _, s := range fileScopes {
		labels = append(labels, s.String())
	}
	for _, s := range projectScopes {
		labels = append(labels, s.String())
	}
	sort.Strings(labels)
	return append([]string{ScopeGroupFile, ScopeGroupProject}, labels...)
}

// ParseScopeSet parses a comma-separated list of scope groups ("file",
// "project") and dispatcher scope labels ("cross-file", "line-pass", ...).
// Tokens are case-insensitive. An empty spec returns a nil set. An unknown
// token is an error, so a typo cannot silently select no rules.
func ParseScopeSet(spec string) (ScopeSet, error) {
	var set ScopeSet
	for _, raw := range strings.Split(spec, ",") {
		token := strings.ToLower(strings.TrimSpace(raw))
		if token == "" {
			continue
		}
		scopes, ok := scopesForToken(token)
		if !ok {
			return nil, fmt.Errorf("unknown rule scope %q (valid: %s)", token, strings.Join(ScopeTokens(), ", "))
		}
		if set == nil {
			set = ScopeSet{}
		}
		for _, s := range scopes {
			set[s] = true
		}
	}
	return set, nil
}

func scopesForToken(token string) ([]api.Scope, bool) {
	switch token {
	case ScopeGroupFile:
		return fileScopes, true
	case ScopeGroupProject:
		// A rule whose scope cannot be derived is not a per-file rule.
		return append([]api.Scope{api.ScopeUnset}, projectScopes...), true
	}
	for _, s := range append(append([]api.Scope{}, fileScopes...), projectScopes...) {
		if s.String() == token {
			return []api.Scope{s}, true
		}
	}
	return nil, false
}

// FilterByScope keeps the rules whose dispatcher scope (RuleScope) is in only,
// when only is non-nil, and not in exclude.
func FilterByScope(in []*api.Rule, only, exclude ScopeSet) []*api.Rule {
	if only == nil && exclude == nil {
		return in
	}
	out := make([]*api.Rule, 0, len(in))
	for _, r := range in {
		scope := RuleScope(r)
		if only != nil && !only[scope] {
			continue
		}
		if exclude[scope] {
			continue
		}
		out = append(out, r)
	}
	return out
}
