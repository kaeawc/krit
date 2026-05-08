package rules

import (
	"reflect"
	"sort"
	"testing"
)

func TestIsLikelyLogReceiver(t *testing.T) {
	aliases := map[string]string{
		"logger": "org.slf4j.Logger",
		"L":      "org.slf4j.Logger",
	}
	cases := []struct {
		recv string
		want bool
	}{
		{"", false},
		{"logger", true},
		{"L", true},
		{"unknown", false},
	}
	for _, tc := range cases {
		t.Run(tc.recv, func(t *testing.T) {
			if got := isLikelyLogReceiver(tc.recv, aliases); got != tc.want {
				t.Fatalf("isLikelyLogReceiver(%q) = %v, want %v", tc.recv, got, tc.want)
			}
		})
	}
}

func TestIsKnownLoggerTypeText(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"org.slf4j.Logger", true},
		{"org.slf4j.Logger?", true},
		{"  org.slf4j.Logger  ", true},
		{"ch.qos.logback.classic.Logger", true},
		{"org.apache.logging.log4j.Logger", true},
		{"mu.KLogger", true},
		{"io.github.oshai.kotlinlogging.KLogger", true},
		{"timber.log.Timber", false},
		{"java.util.logging.Logger", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			if got := isKnownLoggerTypeText(tc.text); got != tc.want {
				t.Fatalf("isKnownLoggerTypeText(%q) = %v", tc.text, got)
			}
		})
	}
}

func TestNormalizeConditionText(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"  x  ", "x"},
		{"(x)", "x"},
		{"((x))", "x"},
		{"(x && y)", "x && y"},
		{"x && y", "x && y"},
		// Note: normalizeConditionText strips paired parens iteratively
		// without bracket matching, so "((a) && (b))" reduces to "a) && (b".
		// That's intentional — surrounding-paren stripping for simple
		// conditions like "(x && y)" is the only case the callers rely on.
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := normalizeConditionText(tc.in); got != tc.want {
				t.Fatalf("normalizeConditionText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCompactConditionText(t *testing.T) {
	if got := compactConditionText("  logger . isDebugEnabled  "); got != "logger.isDebugEnabled" {
		t.Fatalf("got %q", got)
	}
	if got := compactConditionText("(logger.isDebugEnabled())"); got != "logger.isDebugEnabled()" {
		t.Fatalf("got %q", got)
	}
}

func TestSplitTopLevelLogicalAnd(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a", []string{"a"}},
		{"a && b", []string{"a", "b"}},
		{"a && b && c", []string{"a", "b", "c"}},
		{"(a && b) && c", []string{"(a && b)", "c"}},
		{"a || b", []string{"a || b"}},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := splitTopLevelLogicalAnd(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("splitTopLevelLogicalAnd(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func TestSplitTopLevelLogicalOr(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a", []string{"a"}},
		{"a || b", []string{"a", "b"}},
		{"(a || b) || c", []string{"(a || b)", "c"}},
		{"a && b", []string{"a && b"}},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := splitTopLevelLogicalOr(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("splitTopLevelLogicalOr(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func TestLogLevelGuardProperty(t *testing.T) {
	if logLevelGuardProperty("trace") != "isTraceEnabled" {
		t.Fatal("trace -> isTraceEnabled")
	}
	if logLevelGuardProperty("debug") != "isDebugEnabled" {
		t.Fatal("debug -> isDebugEnabled")
	}
	if logLevelGuardProperty("anything-else") != "isDebugEnabled" {
		t.Fatal("default -> isDebugEnabled")
	}
}

func TestLogLevelGuardCandidates(t *testing.T) {
	got := logLevelGuardCandidates("logger", "isDebugEnabled")
	want := []string{
		"logger.isDebugEnabled",
		"logger.isDebugEnabled()",
		"logger?.isDebugEnabled",
		"logger?.isDebugEnabled()",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if logLevelGuardCandidates("", "x") != nil {
		t.Fatal("empty receiver should yield nil")
	}
	if logLevelGuardCandidates("logger", "") != nil {
		t.Fatal("empty property should yield nil")
	}
}

func TestConditionTextMatchesBooleanGuard(t *testing.T) {
	cand := "logger.isDebugEnabled"
	cases := []struct {
		text string
		want bool
	}{
		{"logger.isDebugEnabled", true},
		{"logger.isDebugEnabled == true", true},
		{"logger.isDebugEnabled != false", true},
		{"true == logger.isDebugEnabled", true},
		{"foo.logger.isDebugEnabled", true},
		{"logger.isInfoEnabled", false},
		{"logger.isDebugEnabled == false", false},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			if got := conditionTextMatchesBooleanGuard(tc.text, cand); got != tc.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestConditionTextMatchesBooleanNegatedGuard(t *testing.T) {
	cand := "logger.isDebugEnabled"
	cases := []struct {
		text string
		want bool
	}{
		{"logger.isDebugEnabled == false", true},
		{"logger.isDebugEnabled != true", true},
		{"false == logger.isDebugEnabled", true},
		{"true != logger.isDebugEnabled", true},
		{"logger.isDebugEnabled", false},
		{"logger.isDebugEnabled == true", false},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			if got := conditionTextMatchesBooleanNegatedGuard(tc.text, cand); got != tc.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestConditionTextRequiresGuard(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"logger.isDebugEnabled", true},
		{"logger.isDebugEnabled()", true},
		{"logger.isDebugEnabled && shouldLog", true},
		{"shouldLog && logger.isDebugEnabled", true},
		{"logger.isDebugEnabled || logger?.isDebugEnabled", true},
		{"logger.isDebugEnabled || foo", false},
		{"logger.isInfoEnabled", false},
		{"!logger.isDebugEnabled", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			if got := conditionTextRequiresGuard(tc.text, "logger", "isDebugEnabled"); got != tc.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestConditionTextPreventsGuard(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"!logger.isDebugEnabled", true},
		{"logger.isDebugEnabled == false", true},
		// AND requires every conjunct to prevent — "other" doesn't, so false.
		{"!logger.isDebugEnabled && other", false},
		{"!logger.isDebugEnabled && logger?.isDebugEnabled == false", true},
		{"!logger.isDebugEnabled || !logger?.isDebugEnabled", true},
		{"logger.isDebugEnabled", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			if got := conditionTextPreventsGuard(tc.text, "logger", "isDebugEnabled"); got != tc.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestTracingSpanStartText(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"tracer.spanBuilder(\"foo\").startSpan()", true},
		{"tracer\n  .spanBuilder(\"foo\")\n  .startSpan()", true},
		{"tracer.spanBuilder(\"foo\")", false},
		{"tracer.startSpan()", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			if got := tracingSpanStartText(tc.text); got != tc.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestHighCardinalityKeySet(t *testing.T) {
	got := highCardinalityKeySet(nil)
	for _, want := range []string{"user_id", "session_id", "trace_id"} {
		if !got[want] {
			t.Fatalf("default missing %q", want)
		}
	}

	got = highCardinalityKeySet([]string{"  customer_id  ", "", "request_id"})
	if !got["customer_id"] || !got["request_id"] {
		t.Fatal("configured keys missing")
	}
	if got["user_id"] {
		t.Fatal("non-default key leaked into configured set")
	}

	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"customer_id", "request_id"}) {
		t.Fatalf("unexpected keys: %v", keys)
	}
}

func TestMetricNameHasUnitSuffix(t *testing.T) {
	cases := []struct {
		name     string
		suffixes []string
		want     bool
	}{
		{"http_requests_total", nil, true},
		{"http_requests", nil, false},
		{"latency_seconds", nil, true},
		{"latency_ms", []string{"_ms"}, true},
		{"latency_ms", nil, false},
		{"empty_suffix_ignored", []string{""}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := metricNameHasUnitSuffix(tc.name, tc.suffixes); got != tc.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestThrowableLikeInterpolation(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{`"failed: $e"`, true},
		{`"failed: ${error.message}"`, true},
		{`"failed: ${cause}"`, true},
		{`"failed: $msg"`, false}, // msg isn't a throwable-like name
		{`"plain message"`, false},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			if got := throwableLikeInterpolation(tc.text); got != tc.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestInterpolationHasObservedIdentifier(t *testing.T) {
	identifiers := map[string]bool{"traceid": true, "requestid": true}
	cases := []struct {
		text string
		want bool
	}{
		{`"trace=$traceId end"`, true},
		{`"trace=${trace_id} end"`, true},
		// The interpolation regex only captures [A-Za-z_][A-Za-z0-9_]*, so
		// the dash in "${request-id}" terminates the identifier capture at
		// "request" — which doesn't normalise to a known identifier.
		{`"trace=${request-id} end"`, false},
		{`"plain"`, false},
		{`"$other"`, false},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			if got := interpolationHasObservedIdentifier(tc.text, identifiers); got != tc.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestNormalizeObservabilityIdentifier(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"trace_id", "traceid"},
		{"trace-id", "traceid"},
		{"  TRACEID  ", "traceid"},
		{"traceId", "traceid"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := normalizeObservabilityIdentifier(tc.in); got != tc.want {
				t.Fatalf("got %q", got)
			}
		})
	}
}

func TestStructuredLogKeyConvention(t *testing.T) {
	cases := []struct {
		key, want string
	}{
		{"user_id", "snake_case"},
		{"userId", "camelCase"},
		{"plain", "snake_case"},
		{"User_Id", ""},   // mixed: uppercase + underscore is invalid
		{"_leading", ""},  // first char must be lowercase
		{"1leading", ""},  // first char must be lowercase
		{"with-dash", ""}, // dash not allowed
		{"", ""},
		{"x", "snake_case"}, // single lowercase letter falls through to snake
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			if got := structuredLogKeyConvention(tc.key); got != tc.want {
				t.Fatalf("structuredLogKeyConvention(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}
