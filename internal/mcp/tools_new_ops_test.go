package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRulesSearch verifies rules.search returns hits for a known concept.
func TestRulesSearch(t *testing.T) {
	args, _ := json.Marshal(rulesArgs{Operation: "search", Query: "magic"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "rules", Arguments: args}),
	})
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "MagicNumber") {
		t.Errorf("expected MagicNumber in search hits, got: %s", result.Content[0].Text)
	}
}

// TestRulesSearchLanguageSupportFilter verifies search filters by
// LanguageSupport classification.
func TestRulesSearchLanguageSupportFilter(t *testing.T) {
	args, _ := json.Marshal(rulesArgs{
		Operation: "search",
		LanguageSupport: &languageSupportArg{
			Language: "java",
			Status:   []string{"supported"},
		},
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "rules", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "AddJavascriptInterface") {
		t.Errorf("expected AddJavascriptInterface in java=supported hits, got: %s", text)
	}
}

// TestRulesSearchLanguageSupportNegated verifies the negate flag flips
// LanguageSupport membership.
func TestRulesSearchLanguageSupportNegated(t *testing.T) {
	args, _ := json.Marshal(rulesArgs{
		Operation: "search",
		LanguageSupport: &languageSupportArg{
			Language: "java",
			Status:   []string{"supported"},
			Negate:   true,
		},
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "rules", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	// AddJavascriptInterface is explicitly java=supported and must be excluded
	// from the negated set.
	if strings.Contains(text, "\"name\":\"AddJavascriptInterface\"") {
		t.Errorf("expected AddJavascriptInterface excluded by negate; got: %s", text)
	}
}

// TestRulesSearchMissingQuery verifies search requires a query.
func TestRulesSearchMissingQuery(t *testing.T) {
	args, _ := json.Marshal(rulesArgs{Operation: "search"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "rules", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for missing query")
	}
}

// TestRulesSearchWithoutOracle verifies the capability filter excludes
// every rule with any NeedsOracle* bit when "without: [oracle]" is set.
func TestRulesSearchWithoutOracle(t *testing.T) {
	args, _ := json.Marshal(rulesArgs{Operation: "search", Without: []string{"oracle"}})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "rules", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	var parsed struct {
		Total int `json:"total"`
		Hits  []struct {
			Name         string   `json:"name"`
			Capabilities []string `json:"capabilities"`
		} `json:"hits"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &parsed); err != nil {
		t.Fatalf("unmarshal hits: %v", err)
	}
	if parsed.Total == 0 {
		t.Fatal("expected non-zero rules outside the oracle group")
	}
	for _, hit := range parsed.Hits {
		for _, cap := range hit.Capabilities {
			if strings.HasPrefix(cap, "oracle:") {
				t.Errorf("rule %s leaked through Without:[oracle] with capability %s", hit.Name, cap)
			}
		}
	}
}

// TestRulesSearchNeedsResolver verifies the Needs filter restricts to rules
// that declare every requested capability.
func TestRulesSearchNeedsResolver(t *testing.T) {
	args, _ := json.Marshal(rulesArgs{Operation: "search", Needs: []string{"resolver"}})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "rules", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	var parsed struct {
		Total int `json:"total"`
		Hits  []struct {
			Name         string   `json:"name"`
			Capabilities []string `json:"capabilities"`
		} `json:"hits"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &parsed); err != nil {
		t.Fatalf("unmarshal hits: %v", err)
	}
	if parsed.Total == 0 {
		t.Fatal("expected at least one rule that declares NeedsResolver")
	}
	for _, hit := range parsed.Hits {
		found := false
		for _, cap := range hit.Capabilities {
			if cap == "resolver" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("rule %s passed Needs:[resolver] without resolver capability: %v", hit.Name, hit.Capabilities)
		}
	}
}

// TestRulesSearchUnknownCapability verifies unknown capability labels
// produce an error instead of a silent empty result.
func TestRulesSearchUnknownCapability(t *testing.T) {
	args, _ := json.Marshal(rulesArgs{Operation: "search", Needs: []string{"made-up"}})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "rules", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for unknown capability label")
	}
}

// TestRulesCategories verifies categories returns at least one category with rule counts.
func TestRulesCategories(t *testing.T) {
	args, _ := json.Marshal(rulesArgs{Operation: "categories"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "rules", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	var parsed struct {
		Total      int `json:"total"`
		Categories []struct {
			Name      string `json:"name"`
			RuleCount int    `json:"ruleCount"`
		} `json:"categories"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &parsed); err != nil {
		t.Fatalf("unmarshal categories: %v", err)
	}
	if parsed.Total == 0 {
		t.Error("expected non-zero total categories")
	}
	if len(parsed.Categories) == 0 {
		t.Error("expected at least one category")
	}
	if parsed.Categories[0].RuleCount == 0 {
		t.Error("expected non-zero rule count in first category")
	}
}

// TestRulesConfigure verifies configure produces krit.yml YAML.
func TestRulesConfigure(t *testing.T) {
	active := false
	args, _ := json.Marshal(rulesArgs{
		Operation: "configure",
		Rule:      "MagicNumber",
		Active:    &active,
		Severity:  "warning",
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "rules", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "MagicNumber") {
		t.Error("expected MagicNumber in configure output")
	}
	if !strings.Contains(result.Content[0].Text, "active: false") {
		t.Error("expected 'active: false' in configure YAML")
	}
	if !strings.Contains(result.Content[0].Text, "severity: warning") {
		t.Error("expected 'severity: warning' in configure YAML")
	}
}

// TestFixSuppress verifies suppress emits an @Suppress annotation.
func TestFixSuppress(t *testing.T) {
	args, _ := json.Marshal(fixArgs{
		Operation: "suppress",
		Rule:      "MagicNumber",
		Line:      42,
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "fix", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, `@Suppress(\"MagicNumber\")`) {
		t.Errorf("expected @Suppress annotation, got: %s", result.Content[0].Text)
	}
}

// TestFixSuppressFileScope verifies file-scope suppression emits @file:Suppress.
func TestFixSuppressFileScope(t *testing.T) {
	args, _ := json.Marshal(fixArgs{
		Operation: "suppress",
		Rule:      "MagicNumber",
		Scope:     "file",
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "fix", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, `@file:Suppress(\"MagicNumber\")`) {
		t.Errorf("expected @file:Suppress annotation, got: %s", result.Content[0].Text)
	}
}

// TestFixSuppressUnknownRule verifies suppress rejects unknown rules.
func TestFixSuppressUnknownRule(t *testing.T) {
	args, _ := json.Marshal(fixArgs{
		Operation: "suppress",
		Rule:      "DoesNotExist",
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "fix", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for unknown rule")
	}
}

// TestSymbolsOutline verifies outline returns declarations.
func TestSymbolsOutline(t *testing.T) {
	code := `package com.example

class Foo {
    fun bar(): Int = 1
    private val baz = "hello"
}

fun topLevel() {}
`
	args, _ := json.Marshal(symbolsArgs{Operation: "outline", Code: code, Path: "Foo.kt"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "symbols", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	var parsed struct {
		Total   int `json:"total"`
		Symbols []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &parsed); err != nil {
		t.Fatalf("unmarshal outline: %v", err)
	}
	if parsed.Total == 0 {
		t.Errorf("expected at least one symbol declared, got none. raw: %s", result.Content[0].Text)
	}
	// We expect to see Foo (class) somewhere in the symbol list.
	foundFoo := false
	for _, sym := range parsed.Symbols {
		if sym.Name == "Foo" {
			foundFoo = true
			break
		}
	}
	if !foundFoo {
		t.Errorf("expected to find 'Foo' in outline, got: %s", result.Content[0].Text)
	}
}

// TestAnalyzeImpactBuffer verifies impact mode runs against a code buffer.
func TestAnalyzeImpactBuffer(t *testing.T) {
	code := "fun main() {   \n    val x = 1\n}\n" // trailing whitespace triggers a rule
	args, _ := json.Marshal(analyzeArgs{Mode: "impact", Code: code})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "analyze", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	var parsed struct {
		TotalFindings int `json:"totalFindings"`
		Rules         []struct {
			Rule          string `json:"rule"`
			Findings      int    `json:"findings"`
			FilesAffected int    `json:"filesAffected"`
		} `json:"rules"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &parsed); err != nil {
		t.Fatalf("unmarshal impact: %v", err)
	}
	if parsed.TotalFindings == 0 {
		t.Error("expected non-zero total findings")
	}
}

// TestAnalyzeImpactRequiresInput verifies impact rejects empty input.
func TestAnalyzeImpactRequiresInput(t *testing.T) {
	args, _ := json.Marshal(analyzeArgs{Mode: "impact"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "analyze", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when neither code nor paths provided to impact mode")
	}
}

// TestAnalyzeUnknownMode verifies analyze rejects unknown modes.
func TestAnalyzeUnknownMode(t *testing.T) {
	args, _ := json.Marshal(analyzeArgs{Mode: "bogus", Code: "fun x() {}"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "analyze", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for unknown mode")
	}
}

// TestRuleListFilter_Maturity verifies search returns only rules whose
// Maturity matches the filter.
func TestRuleListFilter_Maturity(t *testing.T) {
	args, _ := json.Marshal(rulesArgs{Operation: "search", Maturity: "experimental"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "rules", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}
	var parsed struct {
		Hits []struct {
			Name     string `json:"name"`
			Maturity string `json:"maturity"`
		} `json:"hits"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &parsed); err != nil {
		t.Fatalf("unmarshal hits: %v", err)
	}
	for _, h := range parsed.Hits {
		if h.Maturity != "experimental" {
			t.Errorf("hit %q has maturity %q, want experimental", h.Name, h.Maturity)
		}
	}
}

// TestRulesCategoriesMaturityBuckets verifies the categories result exposes
// the three maturity buckets with non-negative counts.
func TestRulesCategoriesMaturityBuckets(t *testing.T) {
	args, _ := json.Marshal(rulesArgs{Operation: "categories"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "rules", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}
	var parsed struct {
		Maturities []struct {
			Name      string `json:"name"`
			RuleCount int    `json:"ruleCount"`
		} `json:"maturities"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &parsed); err != nil {
		t.Fatalf("unmarshal maturities: %v", err)
	}
	if len(parsed.Maturities) != 3 {
		t.Fatalf("expected 3 maturity buckets, got %d", len(parsed.Maturities))
	}
	want := map[string]bool{"stable": true, "experimental": true, "deprecated": true}
	for _, b := range parsed.Maturities {
		if !want[b.Name] {
			t.Errorf("unexpected maturity bucket %q", b.Name)
		}
	}
}
