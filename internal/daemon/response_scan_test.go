package daemon

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestScanAnalyzeProjectResponse_MatchesJSONUnmarshal pins the
// byte-identical contract: for every realistic response envelope, the
// scanner must populate Findings/Stats with the same bytes/values
// json.Unmarshal would. The fast path's correctness boundary is here.
func TestScanAnalyzeProjectResponse_MatchesJSONUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		findings string
		stats    AnalyzeProjectStats
	}{
		{
			name:     "empty findings",
			findings: `[]`,
			stats:    AnalyzeProjectStats{FilesScanned: 42, FindingsCount: 0, Cold: true},
		},
		{
			name:     "single finding",
			findings: `[{"file":"A.kt","line":1,"col":1}]`,
			stats:    AnalyzeProjectStats{FindingsCount: 1},
		},
		{
			name:     "findings with quoted commas",
			findings: `[{"file":"A.kt","line":1,"message":"has, comma"},{"file":"B.kt","line":2,"message":"and another, one"}]`,
			stats:    AnalyzeProjectStats{FindingsCount: 2},
		},
		{
			name:     "findings with escaped quotes",
			findings: `[{"file":"weird\"name.kt","line":1,"message":"a \"quoted\" word"}]`,
			stats:    AnalyzeProjectStats{FindingsCount: 1},
		},
		{
			name:     "findings with nested brackets in message",
			findings: `[{"file":"A.kt","line":1,"message":"closure { } and [ ] and ]"}]`,
			stats:    AnalyzeProjectStats{FindingsCount: 1},
		},
		{
			name:     "findings with newline-escape in string",
			findings: `[{"file":"A.kt","line":1,"message":"line1\nline2"}]`,
			stats:    AnalyzeProjectStats{FindingsCount: 1},
		},
		{
			name:     "stats with phase timings",
			findings: `[]`,
			stats: AnalyzeProjectStats{
				FindingsCount:     0,
				WallSeconds:       0.567,
				CodeIndexHit:      true,
				LibraryFactsHit:   true,
				FindingsBundleHit: true,
				PhaseTimingsMs:    PhaseTimingsMs{Parse: 7, Index: 0, Output: 200},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statsBytes, err := json.Marshal(tt.stats)
			if err != nil {
				t.Fatalf("marshal stats: %v", err)
			}
			envelope := []byte(`{"ok":true,"data":{"findings":` + tt.findings + `,"stats":` + string(statsBytes) + `}}`)

			// Reference: full json.Unmarshal.
			var want AnalyzeProjectResult
			outerLine := append([]byte(nil), envelope...)
			outerLine = append(outerLine, '\n')
			var resp Response
			if err := json.Unmarshal(outerLine[:len(outerLine)-1], &resp); err != nil {
				t.Fatalf("setup: unmarshal envelope: %v", err)
			}
			if !resp.OK {
				t.Fatalf("setup: ok=false")
			}
			if err := json.Unmarshal(resp.Data, &want); err != nil {
				t.Fatalf("setup: unmarshal data: %v", err)
			}

			// Fast path.
			var got AnalyzeProjectResult
			handled, err := ScanAnalyzeProjectResponse(outerLine, &got)
			if !handled {
				t.Fatalf("ScanAnalyzeProjectResponse: handled=false (would have fallen back) — input=%s", outerLine)
			}
			if err != nil {
				t.Fatalf("ScanAnalyzeProjectResponse: err=%v", err)
			}
			if !bytes.Equal(got.Findings, want.Findings) {
				t.Errorf("Findings byte mismatch:\n got: %s\nwant: %s", got.Findings, want.Findings)
			}
			if !reflect.DeepEqual(got.Stats, want.Stats) {
				t.Errorf("Stats mismatch:\n got: %+v\nwant: %+v", got.Stats, want.Stats)
			}
		})
	}
}

// TestScanAnalyzeProjectResponse_DispatchProfilePresent confirms the
// fast scanner extracts the optional dispatch_profile object the
// daemon emits when --profile-dispatch is set. Without this branch
// the scanner would either fall back to json.Unmarshal (correct but
// slow) or worse, drop the field on the floor — both of which would
// make the CLI's reportDispatchProfile render an empty distribution
// table.
func TestScanAnalyzeProjectResponse_DispatchProfilePresent(t *testing.T) {
	stats := AnalyzeProjectStats{FindingsCount: 0, WallSeconds: 0.1}
	statsBytes, _ := json.Marshal(stats)
	profile := DispatchProfile{
		WallMs:  42,
		Workers: 4,
		Timings: []FileTiming{{
			Path:     "src/A.kt",
			Size:     128,
			RunMs:    7,
			TotalMs:  9,
			Findings: 1,
		}},
	}
	profileBytes, _ := json.Marshal(profile)
	envelope := []byte(`{"ok":true,"data":{"findings":[],"stats":` + string(statsBytes) +
		`,"dispatch_profile":` + string(profileBytes) + `}}` + "\n")

	var got AnalyzeProjectResult
	handled, err := ScanAnalyzeProjectResponse(envelope, &got)
	if !handled {
		t.Fatalf("dispatch_profile envelope must be handled by fast path; envelope=%s", envelope)
	}
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.DispatchProfile == nil {
		t.Fatalf("DispatchProfile = nil; want populated struct")
	}
	if got.DispatchProfile.Workers != 4 || got.DispatchProfile.WallMs != 42 {
		t.Errorf("DispatchProfile metadata mismatch: got %+v", *got.DispatchProfile)
	}
	if len(got.DispatchProfile.Timings) != 1 || got.DispatchProfile.Timings[0].Path != "src/A.kt" {
		t.Errorf("DispatchProfile.Timings mismatch: got %+v", got.DispatchProfile.Timings)
	}
}

// TestScanAnalyzeProjectResponse_DispatchProfileAbsent pins the
// no-regression contract: when --profile-dispatch is off the
// envelope shape stays identical to the pre-PR shape and
// out.DispatchProfile is nil.
func TestScanAnalyzeProjectResponse_DispatchProfileAbsent(t *testing.T) {
	stats := AnalyzeProjectStats{FindingsCount: 0}
	statsBytes, _ := json.Marshal(stats)
	envelope := []byte(`{"ok":true,"data":{"findings":[],"stats":` + string(statsBytes) + `}}` + "\n")

	got := AnalyzeProjectResult{DispatchProfile: &DispatchProfile{Workers: 99}}
	handled, err := ScanAnalyzeProjectResponse(envelope, &got)
	if !handled {
		t.Fatalf("plain envelope must be handled by fast path; envelope=%s", envelope)
	}
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.DispatchProfile != nil {
		t.Errorf("DispatchProfile = %+v; want nil after scan of profile-less envelope", got.DispatchProfile)
	}
}

// TestScanAnalyzeProjectResponse_ErrorEnvelope handles the
// {"ok":false,"error":"msg"} shape — daemon refuses, CLI surfaces.
func TestScanAnalyzeProjectResponse_ErrorEnvelope(t *testing.T) {
	envelope := []byte(`{"ok":false,"error":"binary hash mismatch"}` + "\n")
	var got AnalyzeProjectResult
	handled, err := ScanAnalyzeProjectResponse(envelope, &got)
	if !handled {
		t.Fatalf("error envelope must be handled by fast path")
	}
	if err == nil || err.Error() != "binary hash mismatch" {
		t.Errorf("error: got %v, want \"binary hash mismatch\"", err)
	}
}

// TestScanAnalyzeProjectResponse_UnknownShapeFallsBack confirms a
// changed wire format (different field order, extra fields, etc.)
// returns handled=false so the caller falls back to json.Unmarshal
// without losing correctness.
func TestScanAnalyzeProjectResponse_UnknownShapeFallsBack(t *testing.T) {
	cases := [][]byte{
		// reordered fields
		[]byte(`{"data":{"findings":[],"stats":{}},"ok":true}` + "\n"),
		// extra outer field
		[]byte(`{"ok":true,"version":1,"data":{"findings":[],"stats":{}}}` + "\n"),
		// stats before findings
		[]byte(`{"ok":true,"data":{"stats":{},"findings":[]}}` + "\n"),
		// whitespace
		[]byte(`{ "ok": true, "data": { "findings": [], "stats": {} } }` + "\n"),
		// truncated
		[]byte(`{"ok":true,"data":{"findings":` + "\n"),
	}
	for i, c := range cases {
		t.Run("case-"+string(rune('a'+i)), func(t *testing.T) {
			var got AnalyzeProjectResult
			handled, err := ScanAnalyzeProjectResponse(c, &got)
			if handled && err == nil {
				t.Errorf("expected handled=false for input %q, got handled=true with no error", c)
			}
		})
	}
}

// TestScanAnalyzeProjectResponse_LargePayload is the production
// shape: 87 k synthetic findings × ~150 B each ≈ 12 MB. Confirms the
// scanner is correct at scale, not just on toy inputs.
func TestScanAnalyzeProjectResponse_LargePayload(t *testing.T) {
	// Build a realistic-ish findings array.
	pieces := make([]string, 0, 87_000)
	for i := 0; i < 87_000; i++ {
		pieces = append(pieces, `{"file":"src/dir/File.kt","line":42,"col":1,"ruleSet":"style","rule":"MaxLineLength","severity":"warning","message":"Line exceeds maximum length","fixable":false,"confidence":0.75}`)
	}
	findings := `[` + strings.Join(pieces, ",") + `]`
	stats := AnalyzeProjectStats{FindingsCount: 87000, WallSeconds: 0.567}
	statsBytes, _ := json.Marshal(stats)
	envelope := []byte(`{"ok":true,"data":{"findings":` + findings + `,"stats":` + string(statsBytes) + `}}` + "\n")

	var got AnalyzeProjectResult
	handled, err := ScanAnalyzeProjectResponse(envelope, &got)
	if !handled {
		t.Fatalf("large payload not handled by fast path")
	}
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got.Findings) != len(findings) {
		t.Errorf("findings length: got %d, want %d", len(got.Findings), len(findings))
	}
	if got.Stats.FindingsCount != 87000 {
		t.Errorf("stats.FindingsCount: got %d, want 87000", got.Stats.FindingsCount)
	}
}
