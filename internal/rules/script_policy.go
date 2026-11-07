package rules

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ScriptPolicyMode selects how rule script scopes apply to .kts files.
type ScriptPolicyMode string

const (
	// ScriptPolicyAuto applies each rule's script scope (api.RuleScriptScope).
	ScriptPolicyAuto ScriptPolicyMode = "auto"
	// ScriptPolicyAll runs every rule on scripts, like detekt before its
	// per-rule excludes.
	ScriptPolicyAll ScriptPolicyMode = "all"
	// ScriptPolicyNone runs no Kotlin-language rule on scripts.
	ScriptPolicyNone ScriptPolicyMode = "none"
)

// ScriptPolicy is the resolved `scripts:` config block.
type ScriptPolicy struct {
	Mode ScriptPolicyMode
	// Include lists rule IDs that run on scripts whatever Mode and the rule's
	// scope say. Exclude lists rule IDs that never run on scripts and wins
	// over Include.
	Include map[string]bool
	Exclude map[string]bool
}

// scriptPolicyState is an installed policy with the per-kind excluded rule
// sets computed from the registry on first use.
type scriptPolicyState struct {
	policy   ScriptPolicy
	once     [scriptKindCount]sync.Once
	excluded [scriptKindCount]map[string]bool
}

// scriptKindCount sizes per-kind arrays; ScriptKind values are small and dense.
const scriptKindCount = int(scanner.ScriptStandalone) + 1

var (
	activeScriptPolicy atomic.Pointer[scriptPolicyState]
	// scriptPolicyGen changes whenever the active policy changes, so caches
	// keyed on it do not serve results for an older policy.
	scriptPolicyGen atomic.Uint64
)

// ParseScriptPolicy reads the top-level `scripts:` block. Unknown modes fall
// back to auto; schema validation reports them separately.
func ParseScriptPolicy(cfg *config.Config) ScriptPolicy {
	p := ScriptPolicy{Mode: ScriptPolicyAuto}
	if cfg == nil {
		return p
	}
	switch mode := ScriptPolicyMode(strings.ToLower(cfg.GetTopLevelString("scripts", "policy", ""))); mode {
	case ScriptPolicyAll, ScriptPolicyNone:
		p.Mode = mode
	}
	p.Include = stringSet(cfg.GetTopLevelStringList("scripts", "include"))
	p.Exclude = stringSet(cfg.GetTopLevelStringList("scripts", "exclude"))
	return p
}

func stringSet(items []string) map[string]bool {
	if len(items) == 0 {
		return nil
	}
	out := make(map[string]bool, len(items))
	for _, s := range items {
		out[s] = true
	}
	return out
}

// SetScriptPolicy installs p as the policy used for every later dispatch.
func SetScriptPolicy(p ScriptPolicy) {
	if p.Mode == "" {
		p.Mode = ScriptPolicyAuto
	}
	activeScriptPolicy.Store(&scriptPolicyState{policy: p})
	scriptPolicyGen.Add(1)
}

func currentScriptPolicyState() *scriptPolicyState {
	if st := activeScriptPolicy.Load(); st != nil {
		return st
	}
	// No policy installed yet: install the default so every caller shares one state.
	activeScriptPolicy.CompareAndSwap(nil, &scriptPolicyState{policy: ScriptPolicy{Mode: ScriptPolicyAuto}})
	return activeScriptPolicy.Load()
}

func currentScriptPolicy() *ScriptPolicy {
	return &currentScriptPolicyState().policy
}

// RuleRunsOnScript reports whether r runs on a script of kind k under the
// active policy. It is true for ScriptNone.
func RuleRunsOnScript(r *api.Rule, k scanner.ScriptKind) bool {
	return ruleRunsOnScript(currentScriptPolicy(), r, k)
}

func ruleRunsOnScript(p *ScriptPolicy, r *api.Rule, k scanner.ScriptKind) bool {
	if k == scanner.ScriptNone || r == nil {
		return true
	}
	if p.Exclude[r.ID] {
		return false
	}
	if p.Include[r.ID] {
		return true
	}
	switch p.Mode {
	case ScriptPolicyAll:
		return true
	case ScriptPolicyNone:
		return false
	default:
		return api.RuleAppliesToScript(r, k)
	}
}

// ScriptExcludedRules returns the IDs of the registered rules that do not run
// on a script of kind k, or nil when k is not a script. The result is computed
// once per installed policy and must not be modified.
func ScriptExcludedRules(k scanner.ScriptKind) map[string]bool {
	if k == scanner.ScriptNone || int(k) >= scriptKindCount {
		return nil
	}
	st := currentScriptPolicyState()
	st.once[k].Do(func() {
		out := make(map[string]bool)
		for _, r := range api.Registry {
			if !ruleRunsOnScript(&st.policy, r, k) {
				out[r.ID] = true
			}
		}
		st.excluded[k] = out
	})
	return st.excluded[k]
}

// ScriptPolicyExcludes returns the rule IDs the script policy removes for
// file, or nil when file is not a Kotlin script. Suppression filters merge it
// so cross-file findings obey the same policy as per-file dispatch.
func ScriptPolicyExcludes(file *scanner.File) map[string]bool {
	return ScriptExcludedRules(file.ScriptKind())
}

// DescribeScriptScope returns a short label for r's script applicability in
// the active policy, for listings.
func DescribeScriptScope(r *api.Rule) string {
	p := currentScriptPolicy()
	switch {
	case p.Exclude[r.ID]:
		return "none"
	case p.Include[r.ID]:
		return "all"
	case p.Mode == ScriptPolicyAll:
		return "all"
	case p.Mode == ScriptPolicyNone:
		return "none"
	default:
		return api.RuleScriptScope(r).String()
	}
}
