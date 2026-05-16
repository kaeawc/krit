package daemon

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// BenchmarkJSONUnmarshal_LargeFindingsEnvelope measures the per-call
// cost of decoding the daemon's response envelope when the embedded
// Data field carries a 30 MB findings JSON. The Kotlin compiler
// corpus warm baseline pays this every analyze even on a bundle
// hit, so a hand-rolled framed-binary wire (Data length prefix +
// raw bytes) would remove this from the critical path.
func BenchmarkJSONUnmarshal_LargeFindingsEnvelope(b *testing.B) {
	// Synthesize a Response payload roughly mirroring the Kotlin
	// compiler corpus warm baseline: ~30 MB of findings JSON inside
	// the daemon's {"ok":true,"data":{"findings":...,"stats":...}}
	// envelope.
	findings := bytes.Repeat([]byte(`{"file":"src/dir/File.kt","line":42,"col":1,"ruleSet":"style","rule":"MaxLineLength","severity":"warning","message":"Line exceeds maximum length","fixable":false,"confidence":0.75},`), 87_000)
	findings = findings[:len(findings)-1] // trim trailing comma
	body := `{"findings":[` + string(findings) + `],"stats":{"ok":true}}`
	resp := Response{OK: true, Data: json.RawMessage(body)}
	envelope, err := json.Marshal(resp)
	if err != nil {
		b.Fatalf("marshal envelope: %v", err)
	}
	envelope = append(envelope, '\n')

	b.ReportMetric(float64(len(envelope))/1024/1024, "MB/op")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var got Response
		if err := json.Unmarshal(envelope[:len(envelope)-1], &got); err != nil {
			b.Fatalf("unmarshal envelope: %v", err)
		}
		_ = got
	}
}

// BenchmarkScanAnalyzeProjectResponse measures the production fast
// path the daemon client uses. Should run ~30x faster than the
// json.Unmarshal baseline above on the same 30 MB envelope.
func BenchmarkScanAnalyzeProjectResponse(b *testing.B) {
	findings := bytes.Repeat([]byte(`{"file":"src/dir/File.kt","line":42,"col":1,"ruleSet":"style","rule":"MaxLineLength","severity":"warning","message":"Line exceeds maximum length","fixable":false,"confidence":0.75},`), 87_000)
	findings = findings[:len(findings)-1]
	body := `{"findings":[` + string(findings) + `],"stats":{"findings_count":87000,"wall_seconds":0.567}}`
	resp := Response{OK: true, Data: json.RawMessage(body)}
	envelope, err := json.Marshal(resp)
	if err != nil {
		b.Fatalf("marshal envelope: %v", err)
	}
	envelope = append(envelope, '\n')

	b.ReportMetric(float64(len(envelope))/1024/1024, "MB/op")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var got AnalyzeProjectResult
		handled, derr := ScanAnalyzeProjectResponse(envelope, &got)
		if !handled {
			b.Fatal("scan fell back")
		}
		if derr != nil {
			b.Fatalf("scan: %v", derr)
		}
	}
}

// BenchmarkRawScan_FindEnvelopeBoundaries is the alternative path: a
// hand-rolled parser that finds the `"data":` boundary and emits the
// raw bytes verbatim, skipping json.Unmarshal entirely.
func BenchmarkRawScan_FindEnvelopeBoundaries(b *testing.B) {
	findings := bytes.Repeat([]byte(`{"file":"src/dir/File.kt","line":42,"col":1,"ruleSet":"style","rule":"MaxLineLength","severity":"warning","message":"Line exceeds maximum length","fixable":false,"confidence":0.75},`), 87_000)
	findings = findings[:len(findings)-1]
	body := `{"findings":[` + string(findings) + `],"stats":{"ok":true}}`
	resp := Response{OK: true, Data: json.RawMessage(body)}
	envelope, err := json.Marshal(resp)
	if err != nil {
		b.Fatalf("marshal envelope: %v", err)
	}

	b.ReportMetric(float64(len(envelope))/1024/1024, "MB/op")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Find `"data":` substring + trailing `}` to extract the embedded
		// payload without a structural JSON parse.
		idx := bytes.Index(envelope, []byte(`"data":`))
		if idx < 0 {
			b.Fatal("envelope shape changed")
		}
		start := idx + len(`"data":`)
		// Ends with `}` immediately followed by the envelope closer.
		// In this synthetic shape the envelope ends in `}}`. Real code
		// would balance braces here.
		_ = strings.Index(string(envelope[start:]), `}}`)
	}
}
