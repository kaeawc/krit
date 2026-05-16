package scan

import (
	"flag"
	"fmt"
	"io"

	"github.com/kaeawc/krit/internal/config"
)

// DepthPreset selects how much compiler-backed analysis krit performs.
//
// Today the dial only toggles the JVM type oracle, but it is the public
// surface for future precision/cost trade-offs (richer expression facts,
// expanded class-table coverage, etc). Adding a new preset here should
// keep the default behavior of `balanced` unchanged.
type DepthPreset string

const (
	// DepthFast skips the JVM type oracle entirely. Source-level type
	// inference still runs.
	DepthFast DepthPreset = "fast"

	// DepthBalanced is the default: source-level inference plus the JVM
	// type oracle.
	DepthBalanced DepthPreset = "balanced"

	// DepthThorough is a forward-looking preset reserved for richer
	// oracle facts (narrow expression types, expanded class-table
	// coverage). Today it behaves identically to balanced.
	DepthThorough DepthPreset = "thorough"
)

// validDepthPresets lists every accepted value. Used for validation
// errors and for keeping the help string in sync.
var validDepthPresets = []DepthPreset{DepthFast, DepthBalanced, DepthThorough}

// parseDepthPreset converts a user-supplied string to a DepthPreset.
// Returns ok=false (and a nil-equivalent default) when the value is
// non-empty but unrecognized; the caller is expected to print a warning
// and fall back to balanced.
func parseDepthPreset(s string) (DepthPreset, bool) {
	switch DepthPreset(s) {
	case DepthFast, DepthBalanced, DepthThorough:
		return DepthPreset(s), true
	case "":
		return "", true
	}
	return "", false
}

// resolveDepthPreset picks the active preset using the standard
// precedence chain: explicit CLI flag > krit.yml analysis.depth >
// DepthBalanced. Unknown values warn to w and fall through to balanced;
// the function never returns the empty string.
func resolveDepthPreset(flagValue string, cfg *config.Config, w io.Writer) DepthPreset {
	// CLI value wins outright when present and parseable.
	if flagValue != "" {
		if p, ok := parseDepthPreset(flagValue); ok && p != "" {
			return p
		}
		fmt.Fprintf(w, "warning: --depth=%q is not one of %s; falling back to %q\n",
			flagValue, validDepthList(), DepthBalanced)
	}
	if cfg != nil {
		if raw := cfg.Analysis().Depth; raw != "" {
			if p, ok := parseDepthPreset(raw); ok && p != "" {
				return p
			}
			fmt.Fprintf(w, "warning: krit.yml analysis.depth=%q is not one of %s; falling back to %q\n",
				raw, validDepthList(), DepthBalanced)
		}
	}
	return DepthBalanced
}

// applyDepthPreset rewrites scanFlags pointers to match the selected
// preset, but only for flags the user did not pass on the command line.
// This preserves the documented precedence: an explicit
// `--no-type-oracle` wins over `analysis.depth: balanced`.
//
// fs must be the FlagSet whose Parse() already ran — applyDepthPreset
// uses fs.Visit to detect explicit-vs-default. Pass nil to apply
// unconditionally (used in unit tests where there is no real FlagSet).
func applyDepthPreset(depth DepthPreset, f *scanFlags, fs *flag.FlagSet) {
	if f == nil {
		return
	}
	explicit := explicitFlagSet(fs)

	switch depth {
	case DepthFast:
		// Skip the JVM oracle. Source-level inference stays on so rules
		// that only need lexical/AST signals continue to fire.
		setBoolIfNotExplicit(f.NoTypeOracle, true, "no-type-oracle", explicit)
	case DepthBalanced, DepthThorough:
		// Both keep the oracle eligible; RunProject receives the
		// additional thorough-only analysis knob after setup.
	}
}

// explicitFlagSet returns the set of flag names the user passed on the
// command line, or nil when fs is nil (tests).
func explicitFlagSet(fs *flag.FlagSet) map[string]bool {
	if fs == nil {
		return nil
	}
	out := make(map[string]bool)
	fs.Visit(func(fl *flag.Flag) { out[fl.Name] = true })
	return out
}

// setBoolIfNotExplicit assigns to ptr only when the user did not pass
// the named flag on the command line. A nil explicit map (test mode)
// counts as "no explicit flags" and applies unconditionally.
func setBoolIfNotExplicit(ptr *bool, value bool, name string, explicit map[string]bool) {
	if ptr == nil {
		return
	}
	if explicit != nil && explicit[name] {
		return
	}
	*ptr = value
}

// validDepthList renders the accepted preset names for warning text.
func validDepthList() string {
	out := ""
	for i, p := range validDepthPresets {
		if i > 0 {
			out += ", "
		}
		out += string(p)
	}
	return out
}
