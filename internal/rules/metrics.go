package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// MetricTimerOutsideBlockRule detects timer.record blocks that don't wrap any
// call work, making the timing measurement effectively meaningless.
type MetricTimerOutsideBlockRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *MetricTimerOutsideBlockRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MetricTimerOutsideBlockRule) shouldFlag(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return false
	}
	if flatCallExpressionName(file, idx) != "record" || !metricsCallLooksRelevant(file, idx, "timer", "metric", "meter", "stopwatch") {
		return false
	}
	lambda := flatCallTrailingLambda(file, idx)
	if lambda == 0 {
		return false
	}
	hasCall := false
	file.FlatWalkNodes(lambda, "call_expression", func(call uint32) {
		if call != idx {
			hasCall = true
		}
	})
	return !hasCall
}

// MetricTagHighCardinalityRule detects metric constructor tag keys that would
// create one time series per user/session/request.
type MetricTagHighCardinalityRule struct {
	FlatDispatchBase
	BaseRule

	Keys []string
}

func (r *MetricTagHighCardinalityRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MetricTagHighCardinalityRule) shouldFlag(file *scanner.File, idx uint32) (string, bool) {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return "", false
	}
	if !metricConstructorNames[flatCallExpressionName(file, idx)] || !metricsCallLooksRelevant(file, idx, "metric", "meter", "registry", "stats") {
		return "", false
	}
	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return "", false
	}
	keys := highCardinalityKeySet(r.Keys)
	for pos := 1; ; pos += 2 {
		arg := flatPositionalValueArgument(file, args, pos)
		if arg == 0 {
			break
		}
		if key := firstHighCardinalityString(file, flatValueArgumentExpression(file, arg), keys); key != "" {
			return key, true
		}
	}
	return "", false
}

var metricConstructorNames = map[string]bool{
	"counter":             true,
	"gauge":               true,
	"timer":               true,
	"summary":             true,
	"distributionSummary": true,
	"longTaskTimer":       true,
}

// MetricNameMissingUnitRule detects metric names without a unit suffix.
type MetricNameMissingUnitRule struct {
	FlatDispatchBase
	BaseRule

	Suffixes []string
}

func (r *MetricNameMissingUnitRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MetricNameMissingUnitRule) shouldFlag(file *scanner.File, idx uint32) (string, bool) {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return "", false
	}
	if !metricConstructorNames[flatCallExpressionName(file, idx)] || !metricsCallLooksRelevant(file, idx, "metric", "meter", "registry", "stats") {
		return "", false
	}
	_, args := flatCallExpressionParts(file, idx)
	first := flatPositionalValueArgument(file, args, 0)
	nameNode := flatValueArgumentExpression(file, first)
	if nameNode == 0 || file.FlatType(nameNode) != "string_literal" || flatContainsStringInterpolation(file, nameNode) {
		return "", false
	}
	name := stringLiteralContent(file, nameNode)
	// Dotted names follow Micrometer/OpenTelemetry conventions, where the unit is separate metadata.
	if name == "" || strings.Contains(name, ".") || metricNameHasUnitSuffix(name, r.Suffixes) {
		return "", false
	}
	return name, true
}

// MetricCounterNotMonotonicRule detects negative counter increments.
type MetricCounterNotMonotonicRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *MetricCounterNotMonotonicRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MetricCounterNotMonotonicRule) shouldFlag(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return false
	}
	if flatCallExpressionName(file, idx) != "increment" || !metricsCallLooksRelevant(file, idx, "counter", "metric", "meter") {
		return false
	}
	_, args := flatCallExpressionParts(file, idx)
	first := flatValueArgumentExpression(file, flatPositionalValueArgument(file, args, 0))
	if first == 0 {
		return false
	}
	if !metricIncrementIsNegativeLiteral(file, first) {
		return false
	}
	// An up/down pairing on the same counter is an in-flight gauge, not a misused monotonic counter.
	return !metricCounterAlsoIncrementedPositively(file, idx)
}

func metricIncrementIsNegativeLiteral(file *scanner.File, expr uint32) bool {
	text := strings.TrimSpace(file.FlatNodeText(expr))
	return strings.HasPrefix(text, "-") && len(text) > 1 && (text[1] >= '0' && text[1] <= '9')
}

// metricCounterKey returns the receiver text of an increment call without whitespace, so
// `inFlight` and `registry.counter("x")` identify the counter.
func metricCounterKey(file *scanner.File, call uint32) string {
	nav, _ := flatCallExpressionParts(file, call)
	if nav == 0 {
		return ""
	}
	text := strings.Join(strings.Fields(file.FlatNodeText(nav)), "")
	text = strings.TrimPrefix(text, "this.")
	for _, sep := range []string{"?.increment", ".increment"} {
		if strings.HasSuffix(text, sep) {
			return strings.TrimSuffix(text, sep)
		}
	}
	return ""
}

// metricCounterAlsoIncrementedPositively reports another increment of the same counter in the file
// with no argument or a non-negative literal amount.
func metricCounterAlsoIncrementedPositively(file *scanner.File, idx uint32) bool {
	key := metricCounterKey(file, idx)
	if key == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "call_expression", func(other uint32) {
		if found || other == idx || flatCallExpressionName(file, other) != "increment" || metricCounterKey(file, other) != key {
			return
		}
		_, args := flatCallExpressionParts(file, other)
		first := flatValueArgumentExpression(file, flatPositionalValueArgument(file, args, 0))
		if first == 0 {
			found = true
			return
		}
		text := strings.TrimSpace(file.FlatNodeText(first))
		if text != "" && text[0] >= '0' && text[0] <= '9' {
			found = true
		}
	})
	return found
}

// metricNameInTestCode reports whether the call sits in a test source set, a test class or an @Test function.
func metricNameInTestCode(file *scanner.File, idx uint32) bool {
	if scanner.IsTestFile(file.Path) {
		return true
	}
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration":
			if androidHasAnnotationFlat(file, cur, "Test") {
				return true
			}
		case "class_declaration", "object_declaration":
			name := extractIdentifierFlat(file, cur)
			if strings.HasSuffix(name, "Test") || strings.HasSuffix(name, "Tests") || strings.HasPrefix(name, "Fake") {
				return true
			}
		}
	}
	return false
}

var metricsLibraryImportPrefixes = []string{
	"io.micrometer.",
	"com.codahale.metrics.",
	"io.dropwizard.metrics",
	"io.prometheus.",
	"io.opentelemetry.",
	"com.timgroup.statsd.",
	"com.datadoghq.",
	"datadog.",
	"kamon.",
	"com.netflix.spectator.",
}

// metricsCallLooksRelevant reports whether the call plausibly targets a
// metrics API: the file imports a metrics library or the receiver name
// contains one of the hints. It keeps generic names such as `timer(...)`,
// `counter(...)` and `record { }` from matching unrelated code.
func metricsCallLooksRelevant(file *scanner.File, idx uint32, hints ...string) bool {
	receiverName := flatReceiverNameFromCall(file, idx)
	receiver := strings.ToLower(receiverName)
	for _, hint := range hints {
		if strings.Contains(receiver, hint) {
			return true
		}
	}
	if receiverName != "" && regexp.MustCompile(`\b`+regexp.QuoteMeta(receiverName)+`\s*:\s*[\w.]*(Counter|Meter|Gauge|Timer|Summary|Registry)\b`).Match(file.Content) {
		return true
	}
	imports := fileFactsCache().Imports(file)
	for _, fqn := range imports.Aliases {
		for _, prefix := range metricsLibraryImportPrefixes {
			if strings.HasPrefix(fqn, prefix) {
				return true
			}
		}
	}
	for w := range imports.Wildcards {
		for _, prefix := range metricsLibraryImportPrefixes {
			if strings.HasPrefix(w, prefix) {
				return true
			}
		}
	}
	return false
}
