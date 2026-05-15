package api

import "sort"

// capabilityLabels lists every Capabilities bit alongside its stable
// string label, in canonical (declaration) order. The labels are the
// canonical wire format used by JSON, SARIF properties, MCP queries,
// and the capability-migration skill. They are intentionally lowercase
// and kebab-cased; oracle fact bits use the "oracle:" prefix so the
// umbrella "oracle" group is easy to identify in queries and filters.
var capabilityLabels = []struct {
	Bit   Capabilities
	Label string
}{
	{NeedsResolver, "resolver"},
	{NeedsModuleIndex, "module-index"},
	{NeedsCrossFile, "cross-file"},
	{NeedsLinePass, "line-pass"},
	{NeedsParsedFiles, "parsed-files"},
	{NeedsManifest, "manifest"},
	{NeedsResources, "resources"},
	{NeedsGradle, "gradle"},
	{NeedsAggregate, "aggregate"},
	{NeedsConcurrent, "concurrent"},
	{NeedsOracleCallTargets, "oracle:call-targets"},
	{NeedsOracleSuspendMarkers, "oracle:suspend-markers"},
	{NeedsOracleExprType, "oracle:expr-type"},
	{NeedsOracleExprAnnotations, "oracle:expr-annotations"},
	{NeedsOracleSupertypes, "oracle:supertypes"},
	{NeedsOracleMembers, "oracle:members"},
	{NeedsOracleMemberSignatures, "oracle:member-signatures"},
	{NeedsOracleClassAnnotations, "oracle:class-annotations"},
	{NeedsOracleMemberAnnotations, "oracle:member-annotations"},
	{NeedsOracleDiagnostics, "oracle:diagnostics"},
	{NeedsOracleLibraryClasses, "oracle:library-classes"},
}

// capabilityGroups maps shorthand group labels to the bitfield they
// expand to. "oracle" is the umbrella for every narrow oracle fact bit;
// filters and queries that name "oracle" match a rule with any of those
// bits set.
var capabilityGroups = map[string]Capabilities{
	"oracle": NeedsOracle,
}

// List returns the canonical, sorted set of capability labels carried
// by this Capabilities bitfield. The result round-trips through
// ParseCapabilities without loss for every bit declared in
// capabilityLabels. Group shorthands like "oracle" are never emitted —
// the narrow oracle fact bits are listed individually so consumers see
// exactly what the rule asked for.
func (c Capabilities) List() []string {
	if c == 0 {
		return nil
	}
	out := make([]string, 0, len(capabilityLabels))
	for _, entry := range capabilityLabels {
		if c&entry.Bit != 0 {
			out = append(out, entry.Label)
		}
	}
	sort.Strings(out)
	return out
}

// ParseCapabilities translates a list of capability labels into the
// matching Capabilities bitfield. Group shorthands (e.g. "oracle") are
// expanded into their constituent narrow bits. Unknown labels are
// reported as the second return value; the bitfield contains only
// recognized labels so callers can choose to surface or ignore the
// unknowns.
func ParseCapabilities(labels []string) (Capabilities, []string) {
	var caps Capabilities
	var unknown []string
	for _, label := range labels {
		if bit, ok := capabilityFromLabel(label); ok {
			caps |= bit
			continue
		}
		unknown = append(unknown, label)
	}
	return caps, unknown
}

// capabilityFromLabel resolves a single label (canonical bit or group
// shorthand) to a Capabilities mask.
func capabilityFromLabel(label string) (Capabilities, bool) {
	for _, entry := range capabilityLabels {
		if entry.Label == label {
			return entry.Bit, true
		}
	}
	if mask, ok := capabilityGroups[label]; ok {
		return mask, true
	}
	return 0, false
}

// KnownCapabilityLabels returns every canonical capability label plus
// the group shorthands, sorted. Used by CLI help, MCP schemas, and
// tests that assert label coverage.
func KnownCapabilityLabels() []string {
	out := make([]string, 0, len(capabilityLabels)+len(capabilityGroups))
	for _, entry := range capabilityLabels {
		out = append(out, entry.Label)
	}
	for label := range capabilityGroups {
		out = append(out, label)
	}
	sort.Strings(out)
	return out
}

// CapabilityProvider exposes a rule's capability bitfield without
// requiring callers to construct a full Rule value. Tests and
// non-registry consumers (migration tooling, schema dumpers) can satisfy
// this interface with a thin wrapper.
type CapabilityProvider interface {
	CapabilitiesList() []string
}

// CapabilitiesList returns the stable, sorted capability labels for
// this rule. Mirrors Capabilities.List but lives on Rule so consumers
// can treat the registry as a CapabilityProvider source.
func (r *Rule) CapabilitiesList() []string {
	if r == nil {
		return nil
	}
	return r.Needs.List()
}

// CapabilityFilter narrows a rule set by required and excluded
// capability labels. A rule matches when every label in Require is
// present on its bitfield and none of the labels in Exclude are. The
// "oracle" group shorthand expands to NeedsOracle (any narrow bit), so
// `Exclude: []string{"oracle"}` removes every rule with any
// NeedsOracle* bit set.
type CapabilityFilter struct {
	Require []string
	Exclude []string
}

// IsZero reports whether the filter has no constraints.
func (f CapabilityFilter) IsZero() bool {
	return len(f.Require) == 0 && len(f.Exclude) == 0
}

// Match reports whether the given capability bitfield satisfies the
// filter. Unknown labels never match — Require with an unknown label
// rejects everything; Exclude with an unknown label is a no-op for
// that label.
func (f CapabilityFilter) Match(c Capabilities) bool {
	for _, label := range f.Require {
		mask, ok := capabilityFromLabel(label)
		if !ok || c&mask == 0 {
			return false
		}
	}
	for _, label := range f.Exclude {
		mask, ok := capabilityFromLabel(label)
		if !ok {
			continue
		}
		if c&mask != 0 {
			return false
		}
	}
	return true
}

// MatchRule is the Rule-typed convenience wrapper around Match.
func (f CapabilityFilter) MatchRule(r *Rule) bool {
	if r == nil {
		return f.IsZero()
	}
	return f.Match(r.Needs)
}
