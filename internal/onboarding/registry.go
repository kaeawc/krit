// Package onboarding implements the shared logic for the krit init
// onboarding flow. It is consumed by both the bash/gum prototype
// (scripts/krit-init.sh, via the krit binary) and by the bubbletea
// TUI in cmd/krit/init.go.
//
// The package has three concerns:
//
//   - Loading and validating config/onboarding/controversial-rules.json
//     (the Registry type below).
//   - Scanning a target directory with each shipped profile and
//     parsing the JSON summary (profiles.go).
//   - Generating a merged krit.yml from a chosen profile plus the
//     user's answers to the questionnaire (writer.go).
//
// The gum script predates this package and still uses yq/jq directly.
// When the two diverge in behavior, this package is the source of
// truth — the TUI is the production implementation.
package onboarding

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Registry is the parsed form of config/onboarding/controversial-rules.json.
type Registry struct {
	SchemaVersion int        `json:"schemaVersion"`
	Description   string     `json:"description"`
	Questions     []Question `json:"questions"`
}

// Question is a single entry in the registry.
type Question struct {
	ID              string          `json:"id"`
	Question        string          `json:"question"`
	Rationale       string          `json:"rationale"`
	Rules           []string        `json:"rules"`
	CascadeFrom     *string         `json:"cascade_from"`
	CascadesTo      []string        `json:"cascades_to"`
	Defaults        map[string]bool `json:"defaults"`
	PositiveFixture *string         `json:"positive_fixture"`
	NegativeFixture *string         `json:"negative_fixture"`
	Kind            string          `json:"kind"`
}

// LoadRegistry reads and parses the controversial-rules registry from
// disk. It validates schemaVersion, cascade references, and that every
// question has per-profile defaults for all four shipped profiles.
func LoadRegistry(path string) (*Registry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading registry %s: %w", path, err)
	}
	var reg Registry
	if err := json.Unmarshal(raw, &reg); err != nil {
		return nil, fmt.Errorf("parsing registry %s: %w", path, err)
	}
	if reg.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported registry schemaVersion %d", reg.SchemaVersion)
	}

	ids := make(map[string]bool, len(reg.Questions))
	for _, q := range reg.Questions {
		if q.ID == "" {
			return nil, fmt.Errorf("registry: question with empty id")
		}
		if ids[q.ID] {
			return nil, fmt.Errorf("registry: duplicate question id %q", q.ID)
		}
		ids[q.ID] = true
		for _, profile := range ProfileNames {
			if _, ok := q.Defaults[profile]; !ok {
				return nil, fmt.Errorf("registry: question %q missing default for profile %q", q.ID, profile)
			}
		}
		if q.Kind != "rule" && q.Kind != "parent" {
			return nil, fmt.Errorf("registry: question %q has invalid kind %q", q.ID, q.Kind)
		}
	}
	for _, q := range reg.Questions {
		if q.CascadeFrom != nil && !ids[*q.CascadeFrom] {
			return nil, fmt.Errorf("registry: question %q cascades from unknown id %q", q.ID, *q.CascadeFrom)
		}
		for _, target := range q.CascadesTo {
			if !ids[target] {
				return nil, fmt.Errorf("registry: question %q cascades to unknown id %q", q.ID, target)
			}
		}
	}
	return &reg, nil
}

// RulesetForQuestion derives the YAML ruleset name for a question
// from its positive_fixture path. Fixture paths follow the
// tests/fixtures/positive/<ruleset>/<Rule>.kt convention. For parent
// questions with no fixture, returns an empty string — callers should
// fall back to an explicit mapping or skip the question.
func (q *Question) RulesetForYAML() string {
	if q.PositiveFixture == nil {
		return ""
	}
	parts := strings.Split(*q.PositiveFixture, "/")
	if len(parts) < 5 {
		return ""
	}
	return parts[3]
}

// Invert reports whether this question's "yes" answer should DISABLE
// its rules rather than enable them. By convention, questions whose
// ID starts with "allow-" are inverted (yes = allow = rule off).
func (q *Question) Invert() bool {
	return strings.HasPrefix(q.ID, "allow-")
}
