package jsonschema

import (
	"encoding/json"
	"testing"
)

func TestObjectMarshal(t *testing.T) {
	s := Object(map[string]*Schema{
		"name": String("the name").WithDefault("foo"),
		"age":  Integer(""),
	}).AdditionalPropertiesFalse().WithRequired("name").WithDescription("a thing")

	out, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got["type"] != "object" {
		t.Errorf("type = %v, want object", got["type"])
	}
	if got["additionalProperties"] != false {
		t.Errorf("additionalProperties = %v, want false", got["additionalProperties"])
	}
	if got["description"] != "a thing" {
		t.Errorf("description = %v", got["description"])
	}
	props, ok := got["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("properties is %T", got["properties"])
	}
	name, ok := props["name"].(map[string]interface{})
	if !ok {
		t.Fatalf("properties.name is %T", props["name"])
	}
	if name["default"] != "foo" {
		t.Errorf("name.default = %v", name["default"])
	}
	req, ok := got["required"].([]interface{})
	if !ok || len(req) != 1 || req[0] != "name" {
		t.Errorf("required = %v", got["required"])
	}
}

func TestArrayItems(t *testing.T) {
	s := Array(String(""), "list of strings")
	out, _ := json.Marshal(s)
	var got map[string]interface{}
	_ = json.Unmarshal(out, &got)
	if got["type"] != "array" {
		t.Errorf("type = %v", got["type"])
	}
	items, ok := got["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("items shape: %T", got["items"])
	}
	if items["type"] != "string" {
		t.Errorf("items.type = %v", items["type"])
	}
}

func TestStringEnum(t *testing.T) {
	s := StringEnum([]string{"a", "b"}, "")
	out, _ := json.Marshal(s)
	var got map[string]interface{}
	_ = json.Unmarshal(out, &got)
	enum, ok := got["enum"].([]interface{})
	if !ok || len(enum) != 2 {
		t.Errorf("enum = %v", got["enum"])
	}
}

func TestPropertyOrderStable(t *testing.T) {
	s := Object(map[string]*Schema{
		"z": String(""),
		"a": String(""),
		"m": String(""),
	})
	a, _ := json.Marshal(s)
	b, _ := json.Marshal(s)
	if string(a) != string(b) {
		t.Errorf("non-deterministic marshal:\n%s\n%s", a, b)
	}
}
