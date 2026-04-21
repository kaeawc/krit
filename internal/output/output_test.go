package output

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/perf"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func testColumnFormatterFindings() []scanner.Finding {
	return []scanner.Finding{
		{
			File:       "b.kt",
			Line:       10,
			Col:        4,
			Severity:   "warning",
			RuleSet:    "style",
			Rule:       "MaxLen",
			Message:    "too long",
			Confidence: 0.95,
		},
		{
			File:     "a.kt",
			Line:     2,
			Col:      1,
			Severity: "error",
			RuleSet:  "naming",
			Rule:     "FunName",
			Message:  `bad "name"`,
		},
		{
			File:     "a.kt",
			Line:     1,
			Col:      3,
			Severity: "warning",
			RuleSet:  "zeta",
			Rule:     "LaterRuleSet",
			Message:  "later ruleset",
		},
		{
			File:     "a.kt",
			Line:     1,
			Col:      3,
			Severity: "warning",
			RuleSet:  "alpha",
			Rule:     "EarlierRule",
			Message:  "earlier ruleset and rule",
		},
		{
			File:     "a.kt",
			Line:     1,
			Col:      2,
			Severity: "warning",
			RuleSet:  "style",
			Rule:     "EarlierColumn",
			Message:  "earlier column",
		},
		{
			File:     "a.kt",
			Line:     1,
			Col:      3,
			Severity: "warning",
			RuleSet:  "style",
			Rule:     "Indent",
			Message:  "wrong indent",
		},
	}
}

func TestFormatPlain_SortsByFileLine(t *testing.T) {
	findings := []scanner.Finding{
		{File: "b.kt", Line: 10, Col: 1, Severity: "warning", RuleSet: "style", Rule: "MaxLen", Message: "too long"},
		{File: "a.kt", Line: 5, Col: 2, Severity: "error", RuleSet: "naming", Rule: "FunName", Message: "bad name"},
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "Indent", Message: "wrong indent"},
	}

	var buf bytes.Buffer
	FormatPlain(&buf, findings)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	// a.kt:1 should come first, then a.kt:5, then b.kt:10
	if !strings.HasPrefix(lines[0], "a.kt:1:") {
		t.Errorf("first line should be a.kt:1, got %s", lines[0])
	}
	if !strings.HasPrefix(lines[1], "a.kt:5:") {
		t.Errorf("second line should be a.kt:5, got %s", lines[1])
	}
	if !strings.HasPrefix(lines[2], "b.kt:10:") {
		t.Errorf("third line should be b.kt:10, got %s", lines[2])
	}
}

func TestFormatPlain_SortsByColumnAndLexicalTieBreakers(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 5, Col: 3, Severity: "warning", RuleSet: "zeta", Rule: "Zulu", Message: "zeta"},
		{File: "a.kt", Line: 5, Col: 3, Severity: "warning", RuleSet: "alpha", Rule: "Beta", Message: "beta"},
		{File: "a.kt", Line: 5, Col: 2, Severity: "warning", RuleSet: "style", Rule: "EarlierColumn", Message: "column"},
		{File: "a.kt", Line: 5, Col: 3, Severity: "warning", RuleSet: "alpha", Rule: "Alpha", Message: "alpha"},
	}

	var buf bytes.Buffer
	FormatPlain(&buf, findings)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")

	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "[style:EarlierColumn]") {
		t.Fatalf("expected earliest column first, got %s", lines[0])
	}
	if !strings.Contains(lines[1], "[alpha:Alpha]") {
		t.Fatalf("expected alpha/Alpha second, got %s", lines[1])
	}
	if !strings.Contains(lines[2], "[alpha:Beta]") {
		t.Fatalf("expected alpha/Beta third, got %s", lines[2])
	}
	if !strings.Contains(lines[3], "[zeta:Zulu]") {
		t.Fatalf("expected zeta/Zulu fourth, got %s", lines[3])
	}
}

func TestFormatPlain_EmptyFindings(t *testing.T) {
	var buf bytes.Buffer
	FormatPlain(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("expected empty output for no findings, got %q", buf.String())
	}
}

func TestFormatPlain_DoesNotMutateInputSlice(t *testing.T) {
	findings := []scanner.Finding{
		{File: "b.kt", Line: 10, Col: 1, Severity: "warning", RuleSet: "style", Rule: "MaxLen", Message: "too long"},
		{File: "a.kt", Line: 1, Col: 1, Severity: "error", RuleSet: "naming", Rule: "FunName", Message: "bad name"},
	}
	original := append([]scanner.Finding(nil), findings...)

	var buf bytes.Buffer
	FormatPlain(&buf, findings)

	if !reflect.DeepEqual(findings, original) {
		t.Fatalf("FormatPlain should not mutate input slice:\nwant: %#v\ngot:  %#v", original, findings)
	}
}

func TestFormatJSON_ValidJSON(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "MaxLen", Message: "too long"},
	}

	var buf bytes.Buffer
	FormatJSON(&buf, findings, "1.0.0", 5, 10, time.Now(), nil, nil, nil, nil)

	var report JSONReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if report.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", report.Version)
	}
	if report.Success {
		t.Error("expected success=false when there are findings")
	}
	if len(report.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(report.Findings))
	}
	if report.Files != 5 {
		t.Errorf("expected 5 files, got %d", report.Files)
	}
	if report.Rules != 10 {
		t.Errorf("expected 10 rules, got %d", report.Rules)
	}
}

func TestFormatJSON_Summary(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "MaxLen", Message: "m1"},
		{File: "b.kt", Line: 2, Col: 1, Severity: "error", RuleSet: "style", Rule: "MaxLen", Message: "m2"},
		{File: "c.kt", Line: 3, Col: 1, Severity: "warning", RuleSet: "naming", Rule: "FunName", Message: "m3"},
	}

	var buf bytes.Buffer
	FormatJSON(&buf, findings, "1.0.0", 3, 2, time.Now(), nil, nil, nil, nil)

	var report JSONReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if report.Summary.Total != 3 {
		t.Errorf("expected total=3, got %d", report.Summary.Total)
	}
	if report.Summary.ByRuleSet["style"] != 2 {
		t.Errorf("expected byRuleSet[style]=2, got %d", report.Summary.ByRuleSet["style"])
	}
	if report.Summary.ByRuleSet["naming"] != 1 {
		t.Errorf("expected byRuleSet[naming]=1, got %d", report.Summary.ByRuleSet["naming"])
	}
	if report.Summary.ByRule["MaxLen"] != 2 {
		t.Errorf("expected byRule[MaxLen]=2, got %d", report.Summary.ByRule["MaxLen"])
	}
	if report.Summary.ByRule["FunName"] != 1 {
		t.Errorf("expected byRule[FunName]=1, got %d", report.Summary.ByRule["FunName"])
	}
}

func TestFormatJSONColumns_MatchesFormatJSON(t *testing.T) {
	findings := []scanner.Finding{
		{
			File:     "b.kt",
			Line:     3,
			Col:      4,
			Severity: "warning",
			RuleSet:  "style",
			Rule:     "FixableIndent",
			Message:  "indent fix",
			Fix: &scanner.Fix{
				StartLine:   3,
				EndLine:     3,
				Replacement: "fixed",
			},
			Confidence: 0.95,
		},
		{
			File:       "a.kt",
			Line:       1,
			Col:        1,
			Severity:   "error",
			RuleSet:    "naming",
			Rule:       "FunName",
			Message:    "bad name",
			Confidence: 0.5,
		},
	}

	activeRules := []*v2.Rule{
		{ID: "FixableIndent", Category: "style", Sev: v2.SeverityWarning, Fix: v2.FixIdiomatic},
	}
	perfTimings := []perf.TimingEntry{{Name: "scan", DurationMs: 42}}
	cacheStats := &cache.CacheStats{HitRate: 0.5, Cached: 1, Total: 2}
	start := time.Unix(1700000000, 0)

	var sliceBuf bytes.Buffer
	FormatJSON(&sliceBuf, findings, "1.2.3", 2, 5, start, perfTimings, activeRules, []string{"exp-a"}, cacheStats)

	columns := scanner.CollectFindings(findings)
	var columnBuf bytes.Buffer
	FormatJSONColumns(&columnBuf, &columns, "1.2.3", 2, 5, start, perfTimings, activeRules, []string{"exp-a"}, cacheStats, nil)

	var want any
	if err := json.Unmarshal(sliceBuf.Bytes(), &want); err != nil {
		t.Fatalf("slice JSON invalid: %v", err)
	}
	var got any
	if err := json.Unmarshal(columnBuf.Bytes(), &got); err != nil {
		t.Fatalf("column JSON invalid: %v", err)
	}
	if wantMap, ok := want.(map[string]any); ok {
		wantMap["durationMs"] = float64(0)
	}
	if gotMap, ok := got.(map[string]any); ok {
		gotMap["durationMs"] = float64(0)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("column JSON mismatch:\nwant: %s\ngot:  %s", sliceBuf.String(), columnBuf.String())
	}
}

func TestFormatPlainColumns_MatchesFormatPlain(t *testing.T) {
	findings := testColumnFormatterFindings()

	var sliceBuf bytes.Buffer
	FormatPlain(&sliceBuf, findings)

	columns := scanner.CollectFindings(findings)
	var columnBuf bytes.Buffer
	FormatPlainColumns(&columnBuf, &columns)

	if got, want := columnBuf.String(), sliceBuf.String(); got != want {
		t.Fatalf("column plain output mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestFormatSARIFColumns_MatchesFormatSARIF(t *testing.T) {
	findings := testColumnFormatterFindings()

	var sliceBuf bytes.Buffer
	FormatSARIF(&sliceBuf, findings, "1.2.3")

	columns := scanner.CollectFindings(findings)
	var columnBuf bytes.Buffer
	FormatSARIFColumns(&columnBuf, &columns, "1.2.3")

	var want any
	if err := json.Unmarshal(sliceBuf.Bytes(), &want); err != nil {
		t.Fatalf("slice SARIF invalid: %v", err)
	}
	var got any
	if err := json.Unmarshal(columnBuf.Bytes(), &got); err != nil {
		t.Fatalf("column SARIF invalid: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("column SARIF mismatch:\nwant: %s\ngot:  %s", sliceBuf.String(), columnBuf.String())
	}
}

func TestFormatCheckstyleColumns_MatchesFormatCheckstyle(t *testing.T) {
	findings := testColumnFormatterFindings()

	var sliceBuf bytes.Buffer
	FormatCheckstyle(&sliceBuf, findings)

	columns := scanner.CollectFindings(findings)
	var columnBuf bytes.Buffer
	FormatCheckstyleColumns(&columnBuf, &columns)

	if got, want := columnBuf.String(), sliceBuf.String(); got != want {
		t.Fatalf("column Checkstyle output mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestFormatJSON_PerfTiming(t *testing.T) {
	findings := []scanner.Finding{}

	// With perf timings
	timings := []perf.TimingEntry{
		{Name: "scan", DurationMs: 42},
	}
	var buf bytes.Buffer
	FormatJSON(&buf, findings, "1.0.0", 0, 0, time.Now(), timings, nil, nil, nil)

	var report JSONReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(report.PerfTiming) != 1 {
		t.Errorf("expected 1 perf timing entry, got %d", len(report.PerfTiming))
	}
	if report.PerfTiming[0].Name != "scan" {
		t.Errorf("expected timing name=scan, got %s", report.PerfTiming[0].Name)
	}

	// Without perf timings (nil)
	buf.Reset()
	FormatJSON(&buf, findings, "1.0.0", 0, 0, time.Now(), nil, nil, nil, nil)

	// perfTiming should be omitted from JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, exists := raw["perfTiming"]; exists {
		t.Error("perfTiming should be omitted when nil")
	}
}

func TestFormatJSON_Experiments(t *testing.T) {
	findings := []scanner.Finding{}
	var buf bytes.Buffer
	FormatJSON(&buf, findings, "1.0.0", 1, 2, time.Now(), nil, nil, []string{"exp-b", "exp-a"}, nil)

	var report JSONReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(report.Experiments) != 2 {
		t.Fatalf("expected 2 experiments, got %d", len(report.Experiments))
	}
	if report.Experiments[0] != "exp-b" || report.Experiments[1] != "exp-a" {
		t.Fatalf("unexpected experiments: %#v", report.Experiments)
	}
}

func TestFormatJSON_CacheStats(t *testing.T) {
	stats := &cache.CacheStats{
		HitRate: 0.75,
		Cached:  3,
		Total:   4,
	}

	var buf bytes.Buffer
	FormatJSON(&buf, nil, "1.0.0", 4, 10, time.Now(), nil, nil, nil, stats)

	var report JSONReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if report.Cache == nil {
		t.Fatal("expected cache stats to be present")
	}
	if report.Cache.HitRate != 0.75 {
		t.Errorf("expected hitRate=0.75, got %f", report.Cache.HitRate)
	}
	if report.Cache.Cached != 3 {
		t.Errorf("expected cached=3, got %d", report.Cache.Cached)
	}
}

func TestFormatSARIF_ValidStructure(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "MaxLen", Message: "too long"},
	}

	var buf bytes.Buffer
	FormatSARIF(&buf, findings, "1.0.0")

	var sarif map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &sarif); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if sarif["$schema"] == nil {
		t.Error("missing $schema")
	}
	if sarif["version"] != "2.1.0" {
		t.Errorf("expected version 2.1.0, got %v", sarif["version"])
	}
	runs, ok := sarif["runs"].([]interface{})
	if !ok || len(runs) == 0 {
		t.Error("expected non-empty runs array")
	}
}

func TestFormatCheckstyle_ValidXML(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "MaxLen", Message: "too long"},
		{File: "b.kt", Line: 5, Col: 3, Severity: "error", RuleSet: "naming", Rule: "FunName", Message: "bad name"},
	}

	var buf bytes.Buffer
	FormatCheckstyle(&buf, findings)
	out := buf.String()

	if !strings.HasPrefix(out, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Error("should start with XML declaration")
	}
	if !strings.Contains(out, "<checkstyle") {
		t.Error("should contain <checkstyle> element")
	}
	if !strings.Contains(out, "<file") {
		t.Error("should contain <file> element")
	}
	if !strings.Contains(out, "<error") {
		t.Error("should contain <error> element")
	}
}

func TestFormatJSON_Confidence(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "MaxLen", Message: "too long", Confidence: 0.95},
		{File: "b.kt", Line: 2, Col: 1, Severity: "error", RuleSet: "naming", Rule: "FunName", Message: "bad name"},
	}

	var buf bytes.Buffer
	FormatJSON(&buf, findings, "1.0.0", 2, 2, time.Now(), nil, nil, nil, nil)

	var report JSONReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(report.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(report.Findings))
	}
	if report.Findings[0].Confidence != 0.95 {
		t.Errorf("expected confidence=0.95, got %f", report.Findings[0].Confidence)
	}
	if report.Findings[1].Confidence != 0 {
		t.Errorf("expected confidence=0 (omitted), got %f", report.Findings[1].Confidence)
	}

	// Verify JSON omits confidence when 0
	var raw map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	rawFindings := raw["findings"].([]interface{})
	first := rawFindings[0].(map[string]interface{})
	if _, ok := first["confidence"]; !ok {
		t.Error("confidence should be present when > 0")
	}
	second := rawFindings[1].(map[string]interface{})
	if _, ok := second["confidence"]; ok {
		t.Error("confidence should be omitted when 0")
	}
}

func TestFormatSARIF_Confidence(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "MaxLen", Message: "too long", Confidence: 0.95},
		{File: "b.kt", Line: 2, Col: 1, Severity: "error", RuleSet: "naming", Rule: "FunName", Message: "bad name"},
	}

	var buf bytes.Buffer
	FormatSARIF(&buf, findings, "1.0.0")

	var sarif map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &sarif); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	runs := sarif["runs"].([]interface{})
	run := runs[0].(map[string]interface{})
	results := run["results"].([]interface{})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First result should have confidence in properties
	first := results[0].(map[string]interface{})
	props, ok := first["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties on first result with confidence")
	}
	if props["confidence"] != 0.95 {
		t.Errorf("expected confidence=0.95, got %v", props["confidence"])
	}

	// Second result should not have properties
	second := results[1].(map[string]interface{})
	if _, ok := second["properties"]; ok {
		t.Error("properties should be absent when confidence is 0")
	}
}

func TestFormatPlain_Confidence(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "MaxLen", Message: "too long", Confidence: 0.95},
		{File: "b.kt", Line: 2, Col: 1, Severity: "error", RuleSet: "naming", Rule: "FunName", Message: "bad name"},
	}

	var buf bytes.Buffer
	FormatPlain(&buf, findings)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "[0.95]") {
		t.Errorf("expected confidence [0.95] in output, got %s", lines[0])
	}
	if strings.Contains(lines[1], "[0.") {
		t.Errorf("confidence should not appear when 0, got %s", lines[1])
	}
}

func TestFormatCheckstyle_Confidence(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "MaxLen", Message: "too long", Confidence: 0.95},
		{File: "b.kt", Line: 5, Col: 3, Severity: "error", RuleSet: "naming", Rule: "FunName", Message: "bad name"},
	}

	var buf bytes.Buffer
	FormatCheckstyle(&buf, findings)
	out := buf.String()

	if !strings.Contains(out, `confidence="0.95"`) {
		t.Error("should contain confidence attribute when > 0")
	}
	// The second error should not have confidence
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.Contains(line, "FunName") && strings.Contains(line, "confidence") {
			t.Error("should not have confidence attribute when 0")
		}
	}
}

func TestXMLEscape_SpecialChars(t *testing.T) {
	input := `Tom & Jerry <"hello"> it's`
	escaped := xmlEscape(input)

	if strings.Contains(escaped, "&") && !strings.Contains(escaped, "&amp;") {
		t.Error("& should be escaped to &amp;")
	}
	if strings.Contains(escaped, "<") && !strings.Contains(escaped, "&lt;") {
		t.Error("< should be escaped to &lt;")
	}
	if strings.Contains(escaped, ">") && !strings.Contains(escaped, "&gt;") {
		t.Error("> should be escaped to &gt;")
	}
	if strings.Contains(escaped, `"`) {
		t.Error(`" should be escaped to &quot;`)
	}
	if strings.Contains(escaped, "'") {
		t.Error("' should be escaped to &apos;")
	}

	// Positive checks
	if !strings.Contains(escaped, "&amp;") {
		t.Error("expected &amp; in output")
	}
	if !strings.Contains(escaped, "&lt;") {
		t.Error("expected &lt; in output")
	}
	if !strings.Contains(escaped, "&gt;") {
		t.Error("expected &gt; in output")
	}
	if !strings.Contains(escaped, "&quot;") {
		t.Error("expected &quot; in output")
	}
	if !strings.Contains(escaped, "&apos;") {
		t.Error("expected &apos; in output")
	}
}

func TestFormatPlain_ErrorSeverityFormat(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "error", RuleSet: "naming", Rule: "ClassNaming", Message: "bad name"},
		{File: "a.kt", Line: 5, Col: 1, Severity: "warning", RuleSet: "style", Rule: "MagicNumber", Message: "magic 42"},
	}
	var buf bytes.Buffer
	FormatPlain(&buf, findings)
	out := buf.String()
	if !strings.Contains(out, "error") {
		t.Error("should contain error severity")
	}
	if !strings.Contains(out, "warning") {
		t.Error("should contain warning severity")
	}
	if !strings.Contains(out, "[naming:ClassNaming]") {
		t.Error("should contain rule reference")
	}
}

func TestFormatJSON_FixableFalseWithoutFix(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "MagicNumber", Message: "magic"},
	}
	var buf bytes.Buffer
	FormatJSON(&buf, findings, "1.0", 1, 1, time.Now(), nil, nil, nil, nil)
	out := buf.String()
	if strings.Contains(out, `"fixable":true`) {
		t.Error("fixable should be false when finding has no Fix")
	}
}

func TestFormatJSON_MultipleFindings(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "R1", Message: "m1"},
		{File: "a.kt", Line: 5, Col: 1, Severity: "error", RuleSet: "naming", Rule: "R2", Message: "m2"},
		{File: "b.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "R3", Message: "m3"},
	}
	var buf bytes.Buffer
	FormatJSON(&buf, findings, "1.0", 1, 1, time.Now(), nil, nil, nil, nil)
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	f := result["findings"].([]interface{})
	if len(f) != 3 {
		t.Errorf("expected 3 findings, got %d", len(f))
	}
	summary := result["summary"].(map[string]interface{})
	total := summary["total"].(float64)
	if total != 3 {
		t.Errorf("expected total 3, got %v", total)
	}
}

func TestFormatJSON_SortsByFileLineAndLexicalTieBreakers(t *testing.T) {
	findings := []scanner.Finding{
		{File: "b.kt", Line: 10, Col: 1, Severity: "warning", RuleSet: "style", Rule: "LastFile", Message: "last"},
		{File: "a.kt", Line: 5, Col: 3, Severity: "warning", RuleSet: "zeta", Rule: "Zulu", Message: "zeta"},
		{File: "a.kt", Line: 5, Col: 3, Severity: "warning", RuleSet: "alpha", Rule: "Beta", Message: "beta"},
		{File: "a.kt", Line: 5, Col: 2, Severity: "warning", RuleSet: "style", Rule: "EarlierColumn", Message: "column"},
		{File: "a.kt", Line: 5, Col: 3, Severity: "warning", RuleSet: "alpha", Rule: "Alpha", Message: "alpha"},
	}

	var buf bytes.Buffer
	FormatJSON(&buf, findings, "1.0", 1, 1, time.Now(), nil, nil, nil, nil)

	var report JSONReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	got := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		got = append(got, finding.File+":"+finding.RuleSet+":"+finding.Rule)
	}
	want := []string{
		"a.kt:style:EarlierColumn",
		"a.kt:alpha:Alpha",
		"a.kt:alpha:Beta",
		"a.kt:zeta:Zulu",
		"b.kt:style:LastFile",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON finding order mismatch:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestFormatSARIF_MultipleRules(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "R1", Message: "m1"},
		{File: "b.kt", Line: 5, Col: 1, Severity: "error", RuleSet: "naming", Rule: "R2", Message: "m2"},
	}
	var buf bytes.Buffer
	FormatSARIF(&buf, findings, "1.0")
	var sarif map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &sarif); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	runs := sarif["runs"].([]interface{})
	run := runs[0].(map[string]interface{})
	results := run["results"].([]interface{})
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	// Check different rule IDs
	r1 := results[0].(map[string]interface{})["ruleId"].(string)
	r2 := results[1].(map[string]interface{})["ruleId"].(string)
	if r1 == r2 {
		t.Error("expected different rule IDs")
	}
}

func TestFormatSARIF_SortsByFileLineAndLexicalTieBreakers(t *testing.T) {
	findings := []scanner.Finding{
		{File: "b.kt", Line: 10, Col: 1, Severity: "warning", RuleSet: "style", Rule: "LastFile", Message: "last"},
		{File: "a.kt", Line: 5, Col: 3, Severity: "warning", RuleSet: "zeta", Rule: "Zulu", Message: "zeta"},
		{File: "a.kt", Line: 5, Col: 3, Severity: "warning", RuleSet: "alpha", Rule: "Beta", Message: "beta"},
		{File: "a.kt", Line: 5, Col: 2, Severity: "warning", RuleSet: "style", Rule: "EarlierColumn", Message: "column"},
		{File: "a.kt", Line: 5, Col: 3, Severity: "warning", RuleSet: "alpha", Rule: "Alpha", Message: "alpha"},
	}

	var buf bytes.Buffer
	FormatSARIF(&buf, findings, "1.0")

	var log sarifLog
	if err := json.Unmarshal(buf.Bytes(), &log); err != nil {
		t.Fatalf("invalid SARIF JSON: %v", err)
	}
	results := log.Runs[0].Results

	got := make([]string, 0, len(results))
	for _, result := range results {
		location := result.Locations[0].PhysicalLocation
		got = append(got, location.ArtifactLocation.URI+":"+result.RuleID)
	}
	want := []string{
		"a.kt:style/EarlierColumn",
		"a.kt:alpha/Alpha",
		"a.kt:alpha/Beta",
		"a.kt:zeta/Zulu",
		"b.kt:style/LastFile",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SARIF result order mismatch:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestFormatCheckstyle_MultipleFiles(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "R1", Message: "m1"},
		{File: "b.kt", Line: 5, Col: 1, Severity: "error", RuleSet: "naming", Rule: "R2", Message: "m2"},
		{File: "a.kt", Line: 10, Col: 1, Severity: "warning", RuleSet: "style", Rule: "R3", Message: "m3"},
	}
	var buf bytes.Buffer
	FormatCheckstyle(&buf, findings)
	out := buf.String()
	// Should have 2 <file> elements (a.kt and b.kt)
	fileCount := strings.Count(out, "<file ")
	if fileCount != 2 {
		t.Errorf("expected 2 file elements, got %d", fileCount)
	}
	// a.kt should have 2 errors, b.kt should have 1
	errorCount := strings.Count(out, "<error ")
	if errorCount != 3 {
		t.Errorf("expected 3 error elements, got %d", errorCount)
	}
}

func TestFormatCheckstyle_SortsByColumnAndLexicalTieBreakers(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 5, Col: 3, Severity: "warning", RuleSet: "zeta", Rule: "Zulu", Message: "zeta"},
		{File: "a.kt", Line: 5, Col: 3, Severity: "warning", RuleSet: "alpha", Rule: "Beta", Message: "beta"},
		{File: "a.kt", Line: 5, Col: 2, Severity: "warning", RuleSet: "style", Rule: "EarlierColumn", Message: "column"},
		{File: "a.kt", Line: 5, Col: 3, Severity: "warning", RuleSet: "alpha", Rule: "Alpha", Message: "alpha"},
	}

	var buf bytes.Buffer
	FormatCheckstyle(&buf, findings)
	out := buf.String()

	order := []string{
		`source="style.EarlierColumn"`,
		`source="alpha.Alpha"`,
		`source="alpha.Beta"`,
		`source="zeta.Zulu"`,
	}
	prev := -1
	for _, needle := range order {
		idx := strings.Index(out, needle)
		if idx < 0 {
			t.Fatalf("missing %s in output:\n%s", needle, out)
		}
		if idx <= prev {
			t.Fatalf("expected %s after prior entry in output:\n%s", needle, out)
		}
		prev = idx
	}
}

func TestFormatCheckstyle_SpecialCharsInMessage(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "R1",
			Message: `Use "HashMap<Int, String>" instead`},
	}
	var buf bytes.Buffer
	FormatCheckstyle(&buf, findings)
	out := buf.String()
	// Angle brackets and quotes should be escaped in XML
	if strings.Contains(out, `message="Use "`) {
		t.Error("unescaped quotes in XML attribute")
	}
	if strings.Contains(out, "<Int") && !strings.Contains(out, "&lt;Int") {
		t.Error("unescaped < in XML attribute")
	}
}

func TestFormatPlain_NoColorWhenNotTerminal(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "error", RuleSet: "style", Rule: "R1", Message: "test"},
	}
	var buf bytes.Buffer
	FormatPlain(&buf, findings) // bytes.Buffer is not a terminal
	out := buf.String()
	if strings.Contains(out, "\033[") {
		t.Error("should not contain ANSI escape codes when writing to non-terminal")
	}
}

func TestFormatSARIF_SpecialCharsInMessage(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "R1",
			Message: "backslash \\ quote \" newline \n tab \t"},
	}
	var buf bytes.Buffer
	FormatSARIF(&buf, findings, "1.0.0")

	var sarif sarifLog
	if err := json.Unmarshal(buf.Bytes(), &sarif); err != nil {
		t.Fatalf("invalid JSON with special chars: %v", err)
	}
	if len(sarif.Runs) == 0 || len(sarif.Runs[0].Results) == 0 {
		t.Fatal("expected at least one run with one result")
	}
	msg := sarif.Runs[0].Results[0].Message.Text
	if !strings.Contains(msg, `\`) {
		t.Error("expected backslash in message")
	}
	if !strings.Contains(msg, `"`) {
		t.Error("expected quote in message")
	}
	if !strings.Contains(msg, "\n") {
		t.Error("expected newline in message")
	}
}

func TestFormatSARIF_EmptyFindings(t *testing.T) {
	var buf bytes.Buffer
	FormatSARIF(&buf, nil, "1.0.0")

	var sarif sarifLog
	if err := json.Unmarshal(buf.Bytes(), &sarif); err != nil {
		t.Fatalf("invalid JSON for empty findings: %v", err)
	}
	if sarif.Version != "2.1.0" {
		t.Errorf("expected version 2.1.0, got %s", sarif.Version)
	}
	if len(sarif.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(sarif.Runs))
	}
	if sarif.Runs[0].Results != nil {
		t.Errorf("expected nil results for empty findings, got %d results", len(sarif.Runs[0].Results))
	}
	// Re-marshal to verify the output is valid JSON with null results
	raw := buf.String()
	if !strings.Contains(raw, `"results"`) {
		t.Error("expected results key in JSON output")
	}
}

func TestFormatSARIF_ConfidenceField(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "R1",
			Message: "with confidence", Confidence: 0.85},
		{File: "b.kt", Line: 2, Col: 1, Severity: "error", RuleSet: "naming", Rule: "R2",
			Message: "without confidence"},
	}
	var buf bytes.Buffer
	FormatSARIF(&buf, findings, "1.0.0")

	var sarif sarifLog
	if err := json.Unmarshal(buf.Bytes(), &sarif); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	results := sarif.Runs[0].Results

	// First result has confidence
	if results[0].Properties == nil {
		t.Fatal("expected properties with confidence on first result")
	}
	if results[0].Properties.Confidence != 0.85 {
		t.Errorf("expected confidence=0.85, got %f", results[0].Properties.Confidence)
	}

	// Second result has no confidence
	if results[1].Properties != nil {
		t.Errorf("expected nil properties on second result, got %+v", results[1].Properties)
	}
}

func TestFormatCheckstyle_EmptyFindings(t *testing.T) {
	var buf bytes.Buffer
	FormatCheckstyle(&buf, nil)
	out := buf.String()

	if !strings.HasPrefix(out, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Error("should start with XML declaration")
	}
	if !strings.Contains(out, "<checkstyle") {
		t.Error("should contain <checkstyle> element")
	}
	if !strings.Contains(out, "</checkstyle>") {
		t.Error("should contain closing </checkstyle> element")
	}
	if strings.Contains(out, "<file") {
		t.Error("should not contain <file> element for empty findings")
	}
	if strings.Contains(out, "<error") {
		t.Error("should not contain <error> element for empty findings")
	}
}

func TestFormatPlain_ZeroColumn(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 5, Col: 0, Severity: "warning", RuleSet: "style", Rule: "R1", Message: "zero col"},
	}
	var buf bytes.Buffer
	FormatPlain(&buf, findings)
	out := buf.String()

	if !strings.Contains(out, "a.kt:5:0:") {
		t.Errorf("expected a.kt:5:0: in output, got %s", out)
	}
	if !strings.Contains(out, "zero col") {
		t.Errorf("expected message in output, got %s", out)
	}
}

func TestFormatJSON_NilActiveRules(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "R1", Message: "test"},
	}
	var buf bytes.Buffer
	// Pass nil for activeRules - should not panic
	FormatJSON(&buf, findings, "1.0.0", 1, 1, time.Now(), nil, nil, nil, nil)

	var report JSONReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON with nil activeRules: %v", err)
	}
	if len(report.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(report.Findings))
	}
	if report.Findings[0].Fixable {
		t.Error("finding should not be fixable with nil activeRules")
	}
}

func TestColumnFormattersMatchSliceFormatters(t *testing.T) {
	findings := testColumnFormatterFindings()
	columns := scanner.CollectFindings(findings)

	t.Run("plain", func(t *testing.T) {
		var want bytes.Buffer
		var got bytes.Buffer
		FormatPlain(&want, append([]scanner.Finding(nil), findings...))
		FormatPlainColumns(&got, &columns)
		if got.String() != want.String() {
			t.Fatalf("plain output mismatch:\nwant:\n%s\ngot:\n%s", want.String(), got.String())
		}
	})

	t.Run("json", func(t *testing.T) {
		start := time.Now()
		var want bytes.Buffer
		var got bytes.Buffer
		FormatJSON(&want, findings, "1.0.0", 3, 7, start, nil, nil, []string{"exp-a"}, nil)
		FormatJSONColumns(&got, &columns, "1.0.0", 3, 7, start, nil, nil, []string{"exp-a"}, nil, nil)

		var wantReport JSONReport
		var gotReport JSONReport
		if err := json.Unmarshal(want.Bytes(), &wantReport); err != nil {
			t.Fatalf("unmarshal slice report: %v", err)
		}
		if err := json.Unmarshal(got.Bytes(), &gotReport); err != nil {
			t.Fatalf("unmarshal column report: %v", err)
		}
		wantReport.DurationMs = 0
		gotReport.DurationMs = 0
		if !reflect.DeepEqual(gotReport, wantReport) {
			t.Fatalf("json report mismatch:\nwant: %#v\ngot: %#v", wantReport, gotReport)
		}
	})

	t.Run("sarif", func(t *testing.T) {
		var want bytes.Buffer
		var got bytes.Buffer
		FormatSARIF(&want, findings, "1.0.0")
		FormatSARIFColumns(&got, &columns, "1.0.0")

		var wantLog sarifLog
		var gotLog sarifLog
		if err := json.Unmarshal(want.Bytes(), &wantLog); err != nil {
			t.Fatalf("unmarshal slice sarif: %v", err)
		}
		if err := json.Unmarshal(got.Bytes(), &gotLog); err != nil {
			t.Fatalf("unmarshal column sarif: %v", err)
		}
		if !reflect.DeepEqual(gotLog, wantLog) {
			t.Fatalf("sarif mismatch:\nwant: %#v\ngot: %#v", wantLog, gotLog)
		}
	})

	t.Run("checkstyle", func(t *testing.T) {
		var want bytes.Buffer
		var got bytes.Buffer
		FormatCheckstyle(&want, append([]scanner.Finding(nil), findings...))
		FormatCheckstyleColumns(&got, &columns)
		if got.String() != want.String() {
			t.Fatalf("checkstyle output mismatch:\nwant:\n%s\ngot:\n%s", want.String(), got.String())
		}
	})
}

func TestFormatCheckstyle_DoesNotMutateInputSlice(t *testing.T) {
	findings := []scanner.Finding{
		{File: "b.kt", Line: 10, Col: 1, Severity: "warning", RuleSet: "style", Rule: "MaxLen", Message: "too long"},
		{File: "a.kt", Line: 1, Col: 1, Severity: "error", RuleSet: "naming", Rule: "FunName", Message: "bad name"},
	}
	original := append([]scanner.Finding(nil), findings...)

	var buf bytes.Buffer
	FormatCheckstyle(&buf, findings)

	if !reflect.DeepEqual(findings, original) {
		t.Fatalf("FormatCheckstyle should not mutate input slice:\nwant: %#v\ngot:  %#v", original, findings)
	}
}

func TestFormatPlain_ConfidenceInNonTerminal(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "R1",
			Message: "with confidence", Confidence: 0.88},
		{File: "b.kt", Line: 2, Col: 1, Severity: "error", RuleSet: "naming", Rule: "R2",
			Message: "no confidence"},
	}
	var buf bytes.Buffer
	FormatPlain(&buf, findings)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	// First line should have confidence
	if !strings.Contains(lines[0], "[0.88]") {
		t.Errorf("expected [0.88] in first line, got %s", lines[0])
	}
	// First line should still have rule info
	if !strings.Contains(lines[0], "[style:R1]") {
		t.Errorf("expected [style:R1] in first line, got %s", lines[0])
	}
	// Second line should NOT have confidence bracket
	if strings.Contains(lines[1], "[0.") {
		t.Errorf("second line should not have confidence, got %s", lines[1])
	}
}

func TestIsTerminal_ReturnsFalseForBuffer(t *testing.T) {
	var buf bytes.Buffer
	if isTerminal(&buf) {
		t.Error("isTerminal should return false for bytes.Buffer")
	}
}
