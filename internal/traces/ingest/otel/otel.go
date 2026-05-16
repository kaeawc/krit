// Package otel parses OpenTelemetry traces (JSON-encoded spans) into
// the canonical traces.Event stream.
//
// Krit ingests batches, not live OTLP gRPC. The accepted shape is the
// OTLP/JSON wire format produced by collectors when exporting to file
// (`file_exporter`), which is also what most OTel SDKs emit when
// configured with `--exporter file`. Spans are reduced to events
// where:
//
//   - top symbol = span.name (or span attribute "code.function" when
//     present — preferred because it carries the resolved symbol).
//   - frame stack = reconstructed by walking parent_span_id upward
//     within the same trace.
//   - timestamp = span.startTimeUnixNano.
//   - kind = call (span start) — span ends become return transitions
//     during reduction by emitting a closing event.
//   - role = derived from span attributes (deployment.environment /
//     service.name suffixes 'test') with fallback to RoleUnknown.
package otel

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/traces"
)

// Document is the relevant subset of OTLP/JSON the parser reads. The
// real OTLP format has more fields (resource attributes, scope, etc.);
// they are tolerated and ignored. Only resourceSpans -> scopeSpans
// -> spans are walked.
type Document struct {
	ResourceSpans []ResourceSpans `json:"resourceSpans"`
}

type ResourceSpans struct {
	Resource   Resource     `json:"resource"`
	ScopeSpans []ScopeSpans `json:"scopeSpans"`
}

type Resource struct {
	Attributes []KeyValue `json:"attributes"`
}

type ScopeSpans struct {
	Spans []Span `json:"spans"`
}

type Span struct {
	TraceID           string     `json:"traceId"`
	SpanID            string     `json:"spanId"`
	ParentSpanID      string     `json:"parentSpanId"`
	Name              string     `json:"name"`
	Kind              int        `json:"kind"`
	StartTimeUnixNano any        `json:"startTimeUnixNano"`
	EndTimeUnixNano   any        `json:"endTimeUnixNano"`
	Attributes        []KeyValue `json:"attributes"`
}

type KeyValue struct {
	Key   string    `json:"key"`
	Value StringVal `json:"value"`
}

// StringVal is the AnyValue subset used: only stringValue is read.
// Other shapes (intValue, boolValue) are tolerated by JSON
// unmarshaling and ignored.
type StringVal struct {
	StringValue string `json:"stringValue"`
}

// Parse decodes OTLP/JSON bytes into events. The caller stamps
// SourceID on each event before passing to Reduce.
func Parse(data []byte) ([]traces.Event, error) {
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("otel: parse: %w", err)
	}
	// Build span lookup so we can walk parents within a trace.
	spans := map[string]Span{}
	resourceRole := map[string]traces.RoleTag{}
	for _, rs := range doc.ResourceSpans {
		role := roleFromAttributes(rs.Resource.Attributes)
		for _, ss := range rs.ScopeSpans {
			for _, sp := range ss.Spans {
				key := sp.TraceID + "/" + sp.SpanID
				spans[key] = sp
				resourceRole[key] = role
			}
		}
	}
	// Iterate spans in (traceID, startTime) order for deterministic
	// event ordering.
	keys := make([]string, 0, len(spans))
	for k := range spans {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		si, sj := spans[keys[i]], spans[keys[j]]
		if si.TraceID != sj.TraceID {
			return si.TraceID < sj.TraceID
		}
		ti, tj := parseUnixNano(si.StartTimeUnixNano), parseUnixNano(sj.StartTimeUnixNano)
		if ti != tj {
			return ti < tj
		}
		return si.SpanID < sj.SpanID
	})

	out := make([]traces.Event, 0, len(spans))
	for _, k := range keys {
		sp := spans[k]
		role := resourceRole[k]
		// Span-level role overrides resource role if present.
		if r := roleFromAttributes(sp.Attributes); r != traces.RoleUnknown {
			role = r
		}
		ev := traces.Event{
			TimestampNS: parseUnixNano(sp.StartTimeUnixNano),
			FrameStack:  frameStack(sp, spans),
			Kind:        traces.KindCall,
			Role:        role,
		}
		if len(ev.FrameStack) > 0 {
			out = append(out, ev)
		}
	}
	return out, nil
}

// frameStack reconstructs the frame stack for sp by walking
// parentSpanId upward inside the same trace. Returns top-of-stack
// first (sp's symbol then its parents).
func frameStack(sp Span, all map[string]Span) []string {
	const maxDepth = 32
	stack := make([]string, 0, 4)
	cur := sp
	for i := 0; i < maxDepth; i++ {
		sym := topSymbol(cur)
		if sym == "" {
			break
		}
		stack = append(stack, sym)
		if cur.ParentSpanID == "" {
			break
		}
		parentKey := cur.TraceID + "/" + cur.ParentSpanID
		parent, ok := all[parentKey]
		if !ok {
			break
		}
		cur = parent
	}
	return stack
}

// topSymbol prefers span attribute "code.function" (the resolved
// callable name) over span.name (which may be a route template or
// human label like "GET /users/:id"). When "code.namespace" is also
// present, the two concatenate as "<namespace>.<function>".
func topSymbol(sp Span) string {
	var fn, ns string
	for _, kv := range sp.Attributes {
		switch kv.Key {
		case "code.function":
			fn = kv.Value.StringValue
		case "code.namespace":
			ns = kv.Value.StringValue
		}
	}
	switch {
	case fn != "" && ns != "":
		return ns + "." + fn
	case fn != "":
		return fn
	default:
		return sp.Name
	}
}

func roleFromAttributes(kvs []KeyValue) traces.RoleTag {
	env := ""
	service := ""
	for _, kv := range kvs {
		switch kv.Key {
		case "deployment.environment":
			env = kv.Value.StringValue
		case "service.name":
			service = kv.Value.StringValue
		case "krit.role":
			if r := traces.RoleTag(kv.Value.StringValue); r != "" {
				return r
			}
		}
	}
	switch strings.ToLower(env) {
	case "test", "testing":
		return traces.RoleTest
	case "production", "prod":
		return traces.RoleRequest
	case "background", "worker":
		return traces.RoleBackground
	}
	if strings.HasSuffix(strings.ToLower(service), "-test") {
		return traces.RoleTest
	}
	return traces.RoleUnknown
}

// parseUnixNano tolerates both numeric and string-encoded
// timestamps. OTLP/JSON often uses strings to avoid JSON's 53-bit
// number precision limit.
func parseUnixNano(v any) int64 {
	switch t := v.(type) {
	case nil:
		return 0
	case float64:
		return int64(t)
	case string:
		if t == "" {
			return 0
		}
		n, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0
		}
		return n
	case json.Number:
		n, err := t.Int64()
		if err != nil {
			return 0
		}
		return n
	}
	return 0
}
