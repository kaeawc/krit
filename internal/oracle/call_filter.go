package oracle

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/hashutil"
)

// CallTargetFilterSummary is the Go-side contract for narrowing JVM
// resolveToCall() work. Enabled=false means at least one active rule needs
// broad call targets, so krit-types must preserve the pre-filter behavior.
type CallTargetFilterSummary struct {
	Enabled              bool
	DisabledBy           []string
	CalleeNames          []string
	TargetFQNs           []string
	LexicalHintsByCallee map[string][]string
	LexicalSkipByCallee  map[string][]string
	RuleProfiles         []CallTargetRuleProfile
	Fingerprint          string
}

// CallTargetRuleProfile carries attribution metadata for instrumentation.
// It does not change the filter fingerprint; the effective JVM resolution
// scope is defined by the summary's CalleeNames, TargetFQNs, and
// LexicalHintsByCallee.
type CallTargetRuleProfile struct {
	RuleID               string              `json:"ruleID"`
	AllCalls             bool                `json:"allCalls"`
	DiscardedOnly        bool                `json:"discardedOnly,omitempty"`
	CalleeNames          []string            `json:"calleeNames,omitempty"`
	TargetFQNs           []string            `json:"targetFQNs,omitempty"`
	LexicalHintsByCallee map[string][]string `json:"lexicalHintsByCallee,omitempty"`
	LexicalSkipByCallee  map[string][]string `json:"lexicalSkipByCallee,omitempty"`
	AnnotatedIdentifiers []string            `json:"annotatedIdentifiers,omitempty"`
	DerivedCalleeNames   []string            `json:"derivedCalleeNames,omitempty"`
	DisabledReason       string              `json:"disabledReason,omitempty"`
}

type callTargetFilterJSON struct {
	Version              int                     `json:"version"`
	Mode                 string                  `json:"mode"`
	CalleeNames          []string                `json:"calleeNames"`
	TargetFQNs           []string                `json:"targetFqns,omitempty"`
	LexicalHintsByCallee map[string][]string     `json:"lexicalHintsByCallee,omitempty"`
	LexicalSkipByCallee  map[string][]string     `json:"lexicalSkipByCallee,omitempty"`
	RuleProfiles         []CallTargetRuleProfile `json:"ruleProfiles,omitempty"`
}

// FinalizeCallTargetFilter sorts, deduplicates, derives simple callee names
// from FQNs, and computes a stable fingerprint. Callers that build summaries
// directly should run this before writing or passing the filter onward.
func FinalizeCallTargetFilter(summary CallTargetFilterSummary) CallTargetFilterSummary {
	if !summary.Enabled {
		sort.Strings(summary.DisabledBy)
		summary.DisabledBy = compactStrings(summary.DisabledBy)
		summary.Fingerprint = ""
		return summary
	}
	for _, fqn := range summary.TargetFQNs {
		if name := simpleCalleeName(fqn); name != "" {
			summary.CalleeNames = append(summary.CalleeNames, name)
			for _, hint := range lexicalHintsForTargetFQN(fqn) {
				summary.LexicalHintsByCallee = appendStringMapValue(summary.LexicalHintsByCallee, name, hint)
			}
		}
	}
	sort.Strings(summary.CalleeNames)
	summary.CalleeNames = compactStrings(summary.CalleeNames)
	sort.Strings(summary.TargetFQNs)
	summary.TargetFQNs = compactStrings(summary.TargetFQNs)
	summary.LexicalHintsByCallee = normalizeStringSliceMap(summary.LexicalHintsByCallee)
	summary.LexicalSkipByCallee = normalizeStringSliceMap(summary.LexicalSkipByCallee)
	for i := range summary.RuleProfiles {
		profile := &summary.RuleProfiles[i]
		sort.Strings(profile.CalleeNames)
		profile.CalleeNames = compactStrings(profile.CalleeNames)
		sort.Strings(profile.TargetFQNs)
		profile.TargetFQNs = compactStrings(profile.TargetFQNs)
		profile.LexicalHintsByCallee = normalizeStringSliceMap(profile.LexicalHintsByCallee)
		profile.LexicalSkipByCallee = normalizeStringSliceMap(profile.LexicalSkipByCallee)
		sort.Strings(profile.AnnotatedIdentifiers)
		profile.AnnotatedIdentifiers = compactStrings(profile.AnnotatedIdentifiers)
		sort.Strings(profile.DerivedCalleeNames)
		profile.DerivedCalleeNames = compactStrings(profile.DerivedCalleeNames)
	}
	sort.Slice(summary.RuleProfiles, func(i, j int) bool {
		return summary.RuleProfiles[i].RuleID < summary.RuleProfiles[j].RuleID
	})
	summary.Fingerprint = fingerprintCallTargetFilter(summary.CalleeNames, summary.TargetFQNs, summary.LexicalHintsByCallee, summary.LexicalSkipByCallee)
	return summary
}

// WriteCallTargetFilterFile writes the callee filter JSON consumed by
// krit-types --call-filter. Disabled summaries return an empty path.
func WriteCallTargetFilterFile(summary CallTargetFilterSummary, tmpDir string) (string, error) {
	if !summary.Enabled {
		return "", nil
	}
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	payload := callTargetFilterJSON{
		Version:              1,
		Mode:                 "calleeNames",
		CalleeNames:          append([]string(nil), summary.CalleeNames...),
		TargetFQNs:           append([]string(nil), summary.TargetFQNs...),
		LexicalHintsByCallee: cloneStringSliceMap(summary.LexicalHintsByCallee),
		LexicalSkipByCallee:  cloneStringSliceMap(summary.LexicalSkipByCallee),
		RuleProfiles:         append([]CallTargetRuleProfile(nil), summary.RuleProfiles...),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(tmpDir, "krit-call-filter-*.json")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	if _, err := f.Write([]byte("\n")); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func simpleCalleeName(fqn string) string {
	fqn = strings.TrimSpace(fqn)
	if fqn == "" {
		return ""
	}
	for _, sep := range []string{"#", "."} {
		if idx := strings.LastIndex(fqn, sep); idx >= 0 && idx < len(fqn)-1 {
			return fqn[idx+1:]
		}
	}
	return fqn
}

func lexicalHintsForTargetFQN(fqn string) []string {
	normalized := strings.TrimSpace(strings.ReplaceAll(fqn, "#", "."))
	if normalized == "" {
		return nil
	}
	callee := simpleCalleeName(normalized)
	prefix := strings.TrimSuffix(normalized, "."+callee)
	if prefix == "" || prefix == normalized {
		return nil
	}

	var hints []string
	hints = append(hints, prefix)
	if idx := strings.LastIndex(prefix, "."); idx > 0 {
		pkg := prefix[:idx]
		receiver := prefix[idx+1:]
		if receiver != "" && receiver[:1] == strings.ToUpper(receiver[:1]) {
			hints = append(hints, pkg)
			hints = append(hints, receiver)
		}
	}
	return hints
}

func fingerprintCallTargetFilter(calleeNames, targetFQNs []string, lexicalHintsByCallee, lexicalSkipByCallee map[string][]string) string {
	h := hashutil.Hasher().New()
	_, _ = h.Write([]byte("call-filter-v2\ncallee\n"))
	for _, name := range calleeNames {
		_, _ = h.Write([]byte(name))
		_, _ = h.Write([]byte{'\n'})
	}
	_, _ = h.Write([]byte("target\n"))
	for _, fqn := range targetFQNs {
		_, _ = h.Write([]byte(fqn))
		_, _ = h.Write([]byte{'\n'})
	}
	_, _ = h.Write([]byte("lexical\n"))
	for _, callee := range sortedStringMapKeys(lexicalHintsByCallee) {
		_, _ = h.Write([]byte(callee))
		_, _ = h.Write([]byte{0})
		for _, hint := range lexicalHintsByCallee[callee] {
			_, _ = h.Write([]byte(hint))
			_, _ = h.Write([]byte{0})
		}
		_, _ = h.Write([]byte{'\n'})
	}
	_, _ = h.Write([]byte("lexical-skip\n"))
	for _, callee := range sortedStringMapKeys(lexicalSkipByCallee) {
		_, _ = h.Write([]byte(callee))
		_, _ = h.Write([]byte{0})
		for _, hint := range lexicalSkipByCallee[callee] {
			_, _ = h.Write([]byte(hint))
			_, _ = h.Write([]byte{0})
		}
		_, _ = h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil)[:8])
}

func appendStringMapValue(m map[string][]string, key, value string) map[string][]string {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return m
	}
	if m == nil {
		m = make(map[string][]string)
	}
	m[key] = append(m[key], value)
	return m
}

func normalizeStringSliceMap(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, len(in))
	for key, values := range in {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		values = append([]string(nil), values...)
		for i := range values {
			values[i] = strings.TrimSpace(values[i])
		}
		sort.Strings(values)
		values = compactStrings(values)
		if len(values) > 0 {
			out[key] = values
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneStringSliceMap(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, len(in))
	for key, values := range in {
		out[key] = append([]string(nil), values...)
	}
	return out
}

func sortedStringMapKeys(in map[string][]string) []string {
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func compactStrings(in []string) []string {
	out := in[:0]
	var last string
	var haveLast bool
	for _, s := range in {
		if s == "" {
			continue
		}
		if haveLast && s == last {
			continue
		}
		out = append(out, s)
		last = s
		haveLast = true
	}
	return out
}

func removeCallTargetFilterFile(path string) {
	if path != "" {
		_ = os.Remove(filepath.Clean(path))
	}
}
