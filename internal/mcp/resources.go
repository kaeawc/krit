package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/kaeawc/krit/internal/rules"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

// resourceDefinitions returns the list of available MCP resources.
func resourceDefinitions() []ResourceDefinition {
	return []ResourceDefinition{
		{
			URI:         "krit://rules",
			Name:        "Rule catalog",
			Description: "JSON list of all krit rules with metadata: name, category, severity, fixable status.",
			MimeType:    "application/json",
		},
		{
			URI:         "krit://schema",
			Name:        "Configuration schema",
			Description: "JSON Schema describing the krit.yml configuration file format.",
			MimeType:    "application/json",
		},
	}
}

// readResource returns the content and MIME type for a resource URI.
func readResource(uri string) (string, string, error) {
	switch uri {
	case "krit://rules":
		return rulesResource()
	case "krit://schema":
		return schemaResource()
	default:
		return "", "", fmt.Errorf("unknown resource: %s", uri)
	}
}

// ruleInfo is the JSON representation of a rule in the rules resource.
type ruleInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	RuleSet     string `json:"ruleSet"`
	Severity    string `json:"severity"`
	Precision   string `json:"precision"`
	Active      bool   `json:"active"`
	Fixable     bool   `json:"fixable"`
	FixLevel    string `json:"fixLevel,omitempty"`
}

// rulesResource returns a JSON list of all registered rules.
func rulesResource() (string, string, error) {
	items := make([]ruleInfo, 0, len(v2.Registry))
	for _, r := range v2.Registry {
		fixLvl, fixable := rules.GetV2FixLevel(r)
		fixLevel := ""
		if fixable {
			fixLevel = fixLvl.String()
		}

		items = append(items, ruleInfo{
			Name:        r.ID,
			Description: r.Description,
			RuleSet:     r.Category,
			Severity:    string(r.Sev),
			Precision:   string(rules.V2RulePrecision(r)),
			Active:      rules.IsDefaultActive(r.ID),
			Fixable:     fixable,
			FixLevel:    fixLevel,
		})
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("marshal rules: %w", err)
	}
	return string(data), "application/json", nil
}

// schemaResource returns a JSON Schema for the krit.yml configuration.
func schemaResource() (string, string, error) {
	schema := map[string]interface{}{
		"$schema":     "http://json-schema.org/draft-07/schema#",
		"title":       "krit configuration",
		"description": "Configuration file for krit Kotlin static analysis tool (krit.yml / .krit.yml).",
		"type":        "object",
		"properties": map[string]interface{}{
			"rules": map[string]interface{}{
				"type":        "object",
				"description": "Rule configuration. Each key is a rule set name containing individual rule settings.",
				"additionalProperties": map[string]interface{}{
					"type": "object",
					"additionalProperties": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"active": map[string]interface{}{
								"type":        "boolean",
								"description": "Whether the rule is enabled.",
							},
							"severity": map[string]interface{}{
								"type":        "string",
								"enum":        []string{"error", "warning", "info"},
								"description": "Override severity for this rule.",
							},
						},
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("marshal schema: %w", err)
	}
	return string(data), "application/json", nil
}
