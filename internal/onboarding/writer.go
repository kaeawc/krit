package onboarding

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Answer represents a resolved yes/no answer to one question, along
// with whether the answer was user-chosen (interactive) or cascaded
// from a parent question.
type Answer struct {
	QuestionID string
	Value      bool // true = yes, false = no
	Cascaded   bool // true if derived from a parent answer
	Parent     string
}

// ResolveAnswers walks the registry in declaration order and fills
// in answers for every question. Interactive callers pass an `ask`
// function that returns the user's yes/no choice for a question;
// non-interactive callers (CI, tests) can pass a closure that returns
// the per-profile default.
//
// Cascade semantics: when a parent question is answered, every child
// whose cascade_from == parent is given a derived answer using the
// per-profile default of the parent's answer-bucket. Children are
// added to the cascaded set so `ask` is never called for them.
//
//	parent answered "yes" → each child uses its strict default
//	parent answered "no"  → each child uses its relaxed default
//
// This matches the gum script's apply_cascades behavior.
func ResolveAnswers(reg *Registry, profile string, ask func(q *Question, defaultYes bool) bool) ([]Answer, error) {
	answers := make([]Answer, 0, len(reg.Questions))
	cascaded := make(map[string]bool)

	// Index by ID for cascade lookups.
	byID := make(map[string]*Question, len(reg.Questions))
	for i := range reg.Questions {
		byID[reg.Questions[i].ID] = &reg.Questions[i]
	}

	for i := range reg.Questions {
		q := &reg.Questions[i]
		if cascaded[q.ID] {
			continue
		}
		def, ok := q.Defaults[profile]
		if !ok {
			return nil, fmt.Errorf("question %q missing default for profile %q", q.ID, profile)
		}

		value := ask(q, def)
		answers = append(answers, Answer{
			QuestionID: q.ID,
			Value:      value,
		})

		// Apply cascades. Children are added to answers too so the
		// writer sees them; they carry Cascaded=true for telemetry.
		for _, childID := range q.CascadesTo {
			child, exists := byID[childID]
			if !exists {
				continue
			}
			bucket := "relaxed"
			if value {
				bucket = "strict"
			}
			derived, ok := child.Defaults[bucket]
			if !ok {
				return nil, fmt.Errorf("child question %q missing default for cascade bucket %q", childID, bucket)
			}
			answers = append(answers, Answer{
				QuestionID: childID,
				Value:      derived,
				Cascaded:   true,
				Parent:     q.ID,
			})
			cascaded[childID] = true
		}
	}
	return answers, nil
}

// Override is one ruleset+rule active state to merge into a profile.
type Override struct {
	Ruleset string
	Rule    string
	Active  bool
}

// ThresholdOverride is one ruleset+rule field value to merge into a
// profile. Used by the TUI threshold-slider phase to adjust numeric
// rule settings like LongMethod.allowedLines.
type ThresholdOverride struct {
	Ruleset string
	Rule    string
	Field   string
	Value   int
}

// BuildOverrides translates resolved answers into override tuples.
// Questions with an empty rules list (parent-only metas) produce no
// overrides on their own — their children do the work.
func BuildOverrides(reg *Registry, answers []Answer) []Override {
	byID := make(map[string]*Question, len(reg.Questions))
	for i := range reg.Questions {
		byID[reg.Questions[i].ID] = &reg.Questions[i]
	}

	var overrides []Override
	for _, a := range answers {
		q := byID[a.QuestionID]
		if q == nil || len(q.Rules) == 0 {
			continue
		}
		active := a.Value
		if q.Invert() {
			active = !active
		}
		ruleset := q.RulesetForYAML()
		if ruleset == "" {
			// Parent question with no fixture; skip — children handle it.
			continue
		}
		for _, rule := range q.Rules {
			overrides = append(overrides, Override{
				Ruleset: ruleset,
				Rule:    rule,
				Active:  active,
			})
		}
	}

	sort.SliceStable(overrides, func(i, j int) bool {
		if overrides[i].Ruleset != overrides[j].Ruleset {
			return overrides[i].Ruleset < overrides[j].Ruleset
		}
		return overrides[i].Rule < overrides[j].Rule
	})
	return overrides
}

// WriteConfigOptions configures a WriteConfig invocation.
type WriteConfigOptions struct {
	ProfileYAML        []byte              // contents of config/profiles/<name>.yml
	ProfileName        string              // profile display name for the header
	Overrides          []Override          // rule on/off overrides from the questionnaire
	ThresholdOverrides []ThresholdOverride // numeric threshold overrides from the slider phase
}

// WriteConfig returns the merged krit.yml bytes. It deep-merges the
// profile template with the override tree, then prepends a header
// comment documenting the profile and override count.
//
// This is the Go equivalent of the gum script's
// `yq eval-all '. as $item ireduce ({}; . * $item)'` invocation.
func WriteConfig(opts WriteConfigOptions) ([]byte, error) {
	var base map[string]interface{}
	if err := yaml.Unmarshal(opts.ProfileYAML, &base); err != nil {
		return nil, fmt.Errorf("parsing profile YAML: %w", err)
	}
	if base == nil {
		base = make(map[string]interface{})
	}

	// Apply overrides. Each override mutates base[ruleset][rule][active].
	for _, o := range opts.Overrides {
		ruleset, _ := base[o.Ruleset].(map[string]interface{})
		if ruleset == nil {
			ruleset = make(map[string]interface{})
			base[o.Ruleset] = ruleset
		}
		rule, _ := ruleset[o.Rule].(map[string]interface{})
		if rule == nil {
			rule = make(map[string]interface{})
			ruleset[o.Rule] = rule
		}
		rule["active"] = o.Active
	}

	// Apply threshold overrides. Same deep-merge pattern, just a
	// different field key.
	for _, to := range opts.ThresholdOverrides {
		ruleset, _ := base[to.Ruleset].(map[string]interface{})
		if ruleset == nil {
			ruleset = make(map[string]interface{})
			base[to.Ruleset] = ruleset
		}
		rule, _ := ruleset[to.Rule].(map[string]interface{})
		if rule == nil {
			rule = make(map[string]interface{})
			ruleset[to.Rule] = rule
		}
		rule[to.Field] = to.Value
	}

	body, err := yaml.Marshal(base)
	if err != nil {
		return nil, fmt.Errorf("marshaling merged config: %w", err)
	}

	var header strings.Builder
	header.WriteString("# Generated by krit init (bubbletea TUI)\n")
	header.WriteString(fmt.Sprintf("# Profile: %s\n", opts.ProfileName))
	header.WriteString(fmt.Sprintf("# Overrides applied: %d\n", len(opts.Overrides)))
	if len(opts.ThresholdOverrides) > 0 {
		header.WriteString(fmt.Sprintf("# Threshold overrides: %d\n", len(opts.ThresholdOverrides)))
	}
	header.WriteString("# Edit this file to change rule state; krit merges it on top\n")
	header.WriteString("# of config/default-krit.yml.\n\n")
	header.Write(body)
	return []byte(header.String()), nil
}

// YAMLUnmarshalMap is a thin wrapper so callers outside this package
// don't need to import gopkg.in/yaml.v3 directly. Used by the TUI to
// read threshold values out of profile templates.
func YAMLUnmarshalMap(data []byte, out *map[string]interface{}) error {
	return yaml.Unmarshal(data, out)
}

// WriteConfigFile is a thin wrapper around WriteConfig that also
// handles backup of an existing krit.yml and 0644 file creation.
func WriteConfigFile(targetDir string, opts WriteConfigOptions) (string, error) {
	configPath := fmt.Sprintf("%s/krit.yml", strings.TrimRight(targetDir, "/"))
	if _, err := os.Stat(configPath); err == nil {
		if err := os.Rename(configPath, configPath+".bak"); err != nil {
			return "", fmt.Errorf("backing up existing krit.yml: %w", err)
		}
	}
	body, err := WriteConfig(opts)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(configPath, body, 0o644); err != nil {
		return "", fmt.Errorf("writing %s: %w", configPath, err)
	}
	return configPath, nil
}
