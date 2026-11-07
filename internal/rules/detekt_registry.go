package rules

import (
	"sync"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// detektRegistryView answers the detekt importer's questions from the rule
// registry, built lazily so it sees every rule registered by init.
type detektRegistryView struct{}

var (
	detektViewOnce sync.Once
	detektViewSets map[string]bool
	detektViewRule map[string]*api.Rule
)

func init() { config.SetRegistryView(detektRegistryView{}) }

func loadDetektView() {
	detektViewOnce.Do(func() {
		detektViewSets = map[string]bool{}
		detektViewRule = make(map[string]*api.Rule, len(api.Registry))
		for _, r := range api.Registry {
			detektViewSets[r.Category] = true
			// The first registration is the primary ID; later ones are aliases.
			if _, dup := detektViewRule[r.ID]; !dup {
				detektViewRule[r.ID] = r
			}
		}
	})
}

func (detektRegistryView) HasRuleSet(set string) bool {
	loadDetektView()
	return detektViewSets[set]
}

func (detektRegistryView) RuleSetOf(rule string) (string, bool) {
	loadDetektView()
	r, ok := detektViewRule[rule]
	if !ok {
		return "", false
	}
	return r.Category, true
}

func (detektRegistryView) CanonicalOption(rule, key string) (string, bool) {
	loadDetektView()
	r, ok := detektViewRule[rule]
	if !ok {
		return "", false
	}
	meta, ok := MetaForRule(r)
	if !ok {
		return "", false
	}
	for _, opt := range meta.Options {
		if opt.Name == key {
			return opt.Name, true
		}
		for _, alias := range opt.Aliases {
			if alias == key {
				return opt.Name, true
			}
		}
	}
	return "", false
}
