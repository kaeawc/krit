package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompareClassifiesExpressionDifferences(t *testing.T) {
	a, err := Parse([]byte(`{"files":{"/r/A.kt":{"expressions":{
		"1:1":{"type":"kotlin.String","nullable":false},
		"2:1":{"type":"kotlin.Int","nullable":false,"callTarget":"f","callTargetResolved":true},
		"3:1":{"type":"kotlin.Int","nullable":false}}},
		"/r/Only.kt":{"expressions":{}}}}`), "/r")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Parse([]byte(`{"files":{"/r/A.kt":{"expressions":{
		"1:1":{"type":"kotlin.String","nullable":false},
		"2:1":{"type":"kotlin.Int","nullable":false,"callTarget":"f","callTargetResolved":true,"callTargetSuspend":true},
		"4:1":{"type":"kotlin.Int","nullable":false}}}}}`), "/r")
	if err != nil {
		t.Fatal(err)
	}
	r := Compare(a, b, "A.kt")
	for kind, want := range map[string]int{KindSuspend: 1, KindOnlyA: 1, KindOnlyB: 1, kindFileOnlyA: 1} {
		if r.Counts[kind] != want {
			t.Errorf("%s = %d, want %d (%v)", kind, r.Counts[kind], want, r.Counts)
		}
	}
	var out bytes.Buffer
	r.Print(&out, 2)
	if !strings.Contains(out.String(), "A.kt 2:1 call-suspend") {
		t.Errorf("detail missing:\n%s", out.String())
	}
}

func TestCompareIdenticalOracles(t *testing.T) {
	doc := []byte(`{"files":{"/r/A.kt":{"expressions":{"1:1":{"type":"kotlin.String","nullable":false}}}}}`)
	a, _ := Parse(doc, "/r")
	b, _ := Parse(doc, "/r")
	r := Compare(a, b, "")
	if len(r.Counts) != 0 || r.Files != 1 || r.SameFiles != 1 {
		t.Fatalf("report = %+v", r)
	}
}
