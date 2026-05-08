package oracle

import (
	"encoding/hex"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/hashutil"
)

// DeclarationProfile gates which KAA symbol fields krit-types extracts per
// class/member. Extracting a smaller profile saves JVM-side time by skipping
// member-scope traversal, type rendering, and annotation walks that no
// active rule consumes.
//
// Default zero-value is treated as "empty" — callers should construct
// profiles through FullDeclarationProfile() or by setting fields explicitly.
type DeclarationProfile struct {
	// ClassShell is the class FQN/kind/visibility/modality header. Always
	// on in practice (the class result cannot be synthesized without it),
	// but carried as a flag for symmetry with the Kotlin-side profile.
	ClassShell bool
	// Supertypes records direct supertype FQNs (excluding kotlin.Any).
	Supertypes bool
	// ClassAnnotations records class-level annotation FQNs.
	ClassAnnotations bool
	// Members gates traversal of symbol.memberScope. When false, the
	// member list is returned empty regardless of the other member flags.
	Members bool
	// MemberSignatures gates return-type rendering and parameter-type
	// extraction for each included member.
	MemberSignatures bool
	// MemberAnnotations records annotation FQNs on each included member.
	MemberAnnotations bool
	// SourceDependencyClosure keeps the library-supertype walk that feeds
	// the oracle cache closure. Disabling it breaks incremental cache
	// invalidation and should be reserved for experiments.
	SourceDependencyClosure bool
}

// FullDeclarationProfile returns the profile that matches pre-profile
// extraction behavior — every field populated. A run with this profile
// writes cache entries with an empty DeclarationProfileFingerprint, so
// they satisfy any subsequent lookup (broader set satisfies narrower).
func FullDeclarationProfile() DeclarationProfile {
	return DeclarationProfile{
		ClassShell:              true,
		Supertypes:              true,
		ClassAnnotations:        true,
		Members:                 true,
		MemberSignatures:        true,
		MemberAnnotations:       true,
		SourceDependencyClosure: true,
	}
}

// IsFull reports whether the profile is equivalent to
// FullDeclarationProfile. Full profiles write unfingerprinted cache
// entries.
func (p DeclarationProfile) IsFull() bool {
	return p == FullDeclarationProfile()
}

// featureNames lists the profile flags in a stable order. Used both for
// fingerprinting and for the krit-types --declaration-profile CLI value.
var declarationProfileFeatureNames = []string{
	"classShell",
	"supertypes",
	"classAnnotations",
	"members",
	"memberSignatures",
	"memberAnnotations",
	"sourceDependencyClosure",
}

const declarationProfileNoneCLIValue = "none"

// enabledFeatures returns the subset of feature names that are enabled,
// sorted in the canonical order above.
func (p DeclarationProfile) enabledFeatures() []string {
	out := make([]string, 0, len(declarationProfileFeatureNames))
	if p.ClassShell {
		out = append(out, "classShell")
	}
	if p.Supertypes {
		out = append(out, "supertypes")
	}
	if p.ClassAnnotations {
		out = append(out, "classAnnotations")
	}
	if p.Members {
		out = append(out, "members")
	}
	if p.MemberSignatures {
		out = append(out, "memberSignatures")
	}
	if p.MemberAnnotations {
		out = append(out, "memberAnnotations")
	}
	if p.SourceDependencyClosure {
		out = append(out, "sourceDependencyClosure")
	}
	return out
}

// CLIValue returns the comma-separated feature list passed to krit-types
// via --declaration-profile. Returns "" for the full profile so callers
// can omit the flag entirely (preserving the pre-profile CLI shape).
func (p DeclarationProfile) CLIValue() string {
	if p.IsFull() {
		return ""
	}
	features := p.enabledFeatures()
	if len(features) == 0 {
		return declarationProfileNoneCLIValue
	}
	return strings.Join(features, ",")
}

// DeclarationProfileSummary is the plumbing wrapper passed through
// InvocationOptions. The fingerprint is empty when the profile is full,
// matching the CallFilterFingerprint convention (empty = broad = satisfies
// any lookup).
type DeclarationProfileSummary struct {
	Profile     DeclarationProfile
	Fingerprint string
}

// FinalizeDeclarationProfile computes the stable fingerprint for a
// profile. Full profiles get an empty fingerprint so their cache entries
// are treated as unfiltered/broad.
func FinalizeDeclarationProfile(profile DeclarationProfile) DeclarationProfileSummary {
	summary := DeclarationProfileSummary{Profile: profile}
	if profile.IsFull() {
		return summary
	}
	features := profile.enabledFeatures()
	sort.Strings(features)
	h := hashutil.Hasher().New()
	_, _ = h.Write([]byte("decl-profile-v1\n"))
	for _, f := range features {
		_, _ = h.Write([]byte(f))
		_, _ = h.Write([]byte{'\n'})
	}
	summary.Fingerprint = hex.EncodeToString(h.Sum(nil)[:8])
	return summary
}

// MergeDeclarationProfiles returns the union of the given profiles. The
// resulting profile contains every feature that any input contains — this
// is the safe default when deriving a profile from a heterogeneous rule
// set, since the broadest-needing rule wins.
func MergeDeclarationProfiles(profiles ...DeclarationProfile) DeclarationProfile {
	out := DeclarationProfile{}
	for _, p := range profiles {
		if p.ClassShell {
			out.ClassShell = true
		}
		if p.Supertypes {
			out.Supertypes = true
		}
		if p.ClassAnnotations {
			out.ClassAnnotations = true
		}
		if p.Members {
			out.Members = true
		}
		if p.MemberSignatures {
			out.MemberSignatures = true
		}
		if p.MemberAnnotations {
			out.MemberAnnotations = true
		}
		if p.SourceDependencyClosure {
			out.SourceDependencyClosure = true
		}
	}
	return out
}

// ParseDeclarationProfile parses the comma-separated feature string used
// on the CLI (and in tests). Unknown feature names are ignored. An empty
// input returns the zero profile, not the full profile — callers that
// want "no flag passed ⇒ full" must check upstream.
func ParseDeclarationProfile(value string) DeclarationProfile {
	profile := DeclarationProfile{}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == declarationProfileNoneCLIValue {
		return profile
	}
	for _, raw := range strings.Split(trimmed, ",") {
		switch strings.TrimSpace(raw) {
		case "classShell":
			profile.ClassShell = true
		case "supertypes":
			profile.Supertypes = true
		case "classAnnotations":
			profile.ClassAnnotations = true
		case "members":
			profile.Members = true
		case "memberSignatures":
			profile.MemberSignatures = true
		case "memberAnnotations":
			profile.MemberAnnotations = true
		case "sourceDependencyClosure":
			profile.SourceDependencyClosure = true
		}
	}
	return profile
}
