// Package jsonschema provides a typed builder for JSON Schema documents.
//
// It is a small subset focused on the shapes krit emits: krit.yml schema
// generation in internal/schema and the MCP tool inputSchema fields in
// internal/mcp. Where the previous code wrote
//
//	map[string]interface{}{"type": "object", "properties": ...}
//
// callers now build a *Schema value directly. *Schema implements
// json.Marshaler via struct tags, so the wire format is unchanged.
package jsonschema

import (
	"encoding/json"
	"sort"
)

// Schema is a JSON Schema fragment. Only fields krit currently emits are
// modelled — extend on demand. All optional fields use omitempty.
//
// Default is interface{} because JSON Schema "default" can be any JSON
// value (int, bool, string, array, object). The interface{} is confined
// to this single field rather than scattered through every literal.
type Schema struct {
	Schema      string `json:"$schema,omitempty"`
	ID          string `json:"$id,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`

	Type   string `json:"type,omitempty"`
	Format string `json:"format,omitempty"`

	// Properties is ordered alphabetically by Marshal. Insertion order is not
	// preserved — callers that need stable output should not rely on it.
	Properties map[string]*Schema `json:"-"`

	// AdditionalProperties as a bool: nil means absent, &false means
	// "no extra keys allowed", &true means "allowed" (rarely emitted).
	AdditionalProperties *bool `json:"additionalProperties,omitempty"`

	Items    *Schema  `json:"items,omitempty"`
	Required []string `json:"required,omitempty"`
	Enum     []string `json:"enum,omitempty"`

	Default interface{} `json:"default,omitempty"`
}

// MarshalJSON renders a Schema with stable, alphabetically-sorted property
// keys so generated schemas produce deterministic output across runs.
func (s *Schema) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("null"), nil
	}

	// Build a side struct so encoding/json walks tags but still emits
	// Properties through our sorted helper.
	type alias Schema
	type wire struct {
		alias
		Properties map[string]json.RawMessage `json:"properties,omitempty"`
	}

	w := wire{alias: alias(*s)}
	if len(s.Properties) > 0 {
		w.Properties = make(map[string]json.RawMessage, len(s.Properties))
		keys := make([]string, 0, len(s.Properties))
		for k := range s.Properties {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b, err := json.Marshal(s.Properties[k])
			if err != nil {
				return nil, err
			}
			w.Properties[k] = b
		}
	}
	return json.Marshal(w)
}

// Object returns a new object Schema with the given properties. Use
// AdditionalPropertiesFalse() / AdditionalPropertiesTrue() to set the
// stricture if desired.
func Object(props map[string]*Schema) *Schema {
	return &Schema{Type: "object", Properties: props}
}

// String returns a new string Schema with an optional description.
func String(description string) *Schema {
	return &Schema{Type: "string", Description: description}
}

// Integer returns a new integer Schema.
func Integer(description string) *Schema {
	return &Schema{Type: "integer", Description: description}
}

// Boolean returns a new boolean Schema.
func Boolean(description string) *Schema {
	return &Schema{Type: "boolean", Description: description}
}

// Number returns a new number Schema (float).
func Number(description string) *Schema {
	return &Schema{Type: "number", Description: description}
}

// Array returns a new array Schema whose items match the given Schema.
func Array(item *Schema, description string) *Schema {
	return &Schema{Type: "array", Items: item, Description: description}
}

// StringEnum returns a new string Schema constrained to the given enum
// values.
func StringEnum(values []string, description string) *Schema {
	return &Schema{Type: "string", Enum: values, Description: description}
}

// AdditionalPropertiesFalse sets additionalProperties:false on s and
// returns s for chaining.
func (s *Schema) AdditionalPropertiesFalse() *Schema {
	f := false
	s.AdditionalProperties = &f
	return s
}

// WithRequired sets the required field list and returns s for chaining.
func (s *Schema) WithRequired(names ...string) *Schema {
	s.Required = names
	return s
}

// WithDefault attaches a default value and returns s for chaining.
func (s *Schema) WithDefault(v interface{}) *Schema {
	s.Default = v
	return s
}

// WithDescription attaches a description and returns s for chaining.
func (s *Schema) WithDescription(d string) *Schema {
	s.Description = d
	return s
}

// WithTitle attaches a title and returns s for chaining.
func (s *Schema) WithTitle(t string) *Schema {
	s.Title = t
	return s
}

// WithSchemaURI attaches a $schema reference and returns s for chaining.
func (s *Schema) WithSchemaURI(uri string) *Schema {
	s.Schema = uri
	return s
}

// WithID attaches a $id reference and returns s for chaining.
func (s *Schema) WithID(id string) *Schema {
	s.ID = id
	return s
}

// ToMap converts the schema to a generic map[string]interface{} suitable
// for callers that still consume the legacy untyped representation.
// Producers should prefer working with *Schema directly and let
// json.Marshal handle serialisation.
func (s *Schema) ToMap() map[string]interface{} {
	if s == nil {
		return nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}
