// Package changelog generates a markdown changelog of rule introductions
// from the v2 rule registry. The output groups rules by Rule.IntroducedIn
// so release notes can be produced deterministically from rule metadata
// rather than scraped from git history.
package changelog

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// Entry is one rule's contribution to a release group.
type Entry struct {
	ID                    string
	Category              string
	Description           string
	EnabledByDefaultSince string
}

// Group is the set of rules introduced in a single krit version.
type Group struct {
	Version string
	Entries []Entry
}

// Snapshot is a minimal view of the rule metadata the changelog needs.
// The fields mirror RuleDescriptor / api.Rule but the snapshot is
// detached from the global registry so tests can supply synthetic data.
type Snapshot struct {
	ID                    string
	Category              string
	Description           string
	IntroducedIn          string
	EnabledByDefaultSince string
}

// GroupByVersion groups rules into version buckets sorted newest-first.
// Within a version, entries are ordered by category then ID for stable
// output. Rules with an empty IntroducedIn are bucketed under
// "unreleased" so the function never silently drops a rule.
func GroupByVersion(rules []Snapshot) []Group {
	byVersion := map[string][]Entry{}
	for _, r := range rules {
		version := r.IntroducedIn
		if version == "" {
			version = "unreleased"
		}
		byVersion[version] = append(byVersion[version], Entry{
			ID:                    r.ID,
			Category:              r.Category,
			Description:           r.Description,
			EnabledByDefaultSince: r.EnabledByDefaultSince,
		})
	}

	versions := make([]string, 0, len(byVersion))
	for v := range byVersion {
		versions = append(versions, v)
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j]) > 0
	})

	out := make([]Group, 0, len(versions))
	for _, v := range versions {
		entries := byVersion[v]
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Category != entries[j].Category {
				return entries[i].Category < entries[j].Category
			}
			return entries[i].ID < entries[j].ID
		})
		out = append(out, Group{Version: v, Entries: entries})
	}
	return out
}

// Render produces a markdown changelog from grouped entries. When
// versionLimit is positive only the most-recent N groups are emitted.
func Render(groups []Group, versionLimit int) string {
	if versionLimit > 0 && len(groups) > versionLimit {
		groups = groups[:versionLimit]
	}
	var sb strings.Builder
	sb.WriteString("# Rule Changelog\n\n")
	sb.WriteString("Auto-generated from rule metadata (Rule.IntroducedIn). Do not edit by hand.\n")
	for _, g := range groups {
		fmt.Fprintf(&sb, "\n## %s\n\n", versionHeader(g.Version))
		for _, e := range g.Entries {
			defaultTag := ""
			if e.EnabledByDefaultSince != "" && e.EnabledByDefaultSince != g.Version {
				defaultTag = fmt.Sprintf(" (default since %s)", e.EnabledByDefaultSince)
			}
			fmt.Fprintf(&sb, "- **%s** [%s] — %s%s\n", e.ID, e.Category, e.Description, defaultTag)
		}
	}
	return sb.String()
}

// FromRegistry builds the snapshot list from the live api.Registry,
// resolving each rule through MetaForRule via the supplied lookup so
// the changelog package itself does not import the rules package
// (which would create an import cycle).
type MetaLookup func(*api.Rule) (api.RuleDescriptor, bool)

// SnapshotRegistry returns a snapshot list for the registry using lookup
// to resolve descriptor fields. Rules without a descriptor are skipped.
func SnapshotRegistry(registry []*api.Rule, lookup MetaLookup) []Snapshot {
	out := make([]Snapshot, 0, len(registry))
	for _, r := range registry {
		if r == nil {
			continue
		}
		meta, ok := lookup(r)
		if !ok {
			continue
		}
		out = append(out, Snapshot{
			ID:                    meta.ID,
			Category:              meta.RuleSet,
			Description:           meta.Description,
			IntroducedIn:          meta.IntroducedIn,
			EnabledByDefaultSince: meta.EnabledByDefaultSince,
		})
	}
	return out
}

// versionHeader returns "vX.Y.Z" for a numeric version, or the literal
// label (e.g. "unreleased") for non-version buckets.
func versionHeader(v string) string {
	if v == "" || v == "unreleased" {
		return "Unreleased"
	}
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

// compareVersions returns +1 if a > b, -1 if a < b, 0 if equal. Non-numeric
// version strings (e.g. "unreleased") sort newer than any numeric version
// so they appear at the top of the changelog.
func compareVersions(a, b string) int {
	aTokens, aOK := parseVersion(a)
	bTokens, bOK := parseVersion(b)
	switch {
	case !aOK && !bOK:
		return strings.Compare(a, b)
	case !aOK:
		return 1
	case !bOK:
		return -1
	}
	for i := 0; i < len(aTokens) || i < len(bTokens); i++ {
		var ai, bi int
		if i < len(aTokens) {
			ai = aTokens[i]
		}
		if i < len(bTokens) {
			bi = bTokens[i]
		}
		if ai != bi {
			if ai > bi {
				return 1
			}
			return -1
		}
	}
	return 0
}

// parseVersion splits "0.42.0" or "v0.42.0" into its numeric components.
// Returns ok=false on any non-numeric component so callers can sort
// unparseable strings separately.
func parseVersion(v string) ([]int, bool) {
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return nil, false
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}
