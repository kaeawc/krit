package javafacts

import "testing"

func TestFactsFingerprint_StableForEqualValues(t *testing.T) {
	a := &Facts{Version: Version, Calls: []CallFact{{File: "A.java", Line: 1, Callee: "x"}}}
	b := &Facts{Version: Version, Calls: []CallFact{{File: "A.java", Line: 1, Callee: "x"}}}
	if a.Fingerprint() != b.Fingerprint() {
		t.Fatal("equal Facts produced different fingerprints")
	}
}

func TestFactsFingerprint_NilDistinct(t *testing.T) {
	var nilFacts *Facts
	zero := &Facts{}
	full := &Facts{Version: Version, Classes: []ClassFact{{Name: "C"}}}
	if nilFacts.Fingerprint() == zero.Fingerprint() {
		t.Error("nil Facts fingerprint matches zero Facts")
	}
	if zero.Fingerprint() == full.Fingerprint() {
		t.Error("zero Facts fingerprint matches non-empty Facts")
	}
}

func TestFactsFingerprint_SensitiveToCalls(t *testing.T) {
	base := &Facts{Version: Version, Calls: []CallFact{{File: "A.java", Line: 1, Callee: "x"}}}
	tweaked := &Facts{Version: Version, Calls: []CallFact{{File: "A.java", Line: 2, Callee: "x"}}}
	if base.Fingerprint() == tweaked.Fingerprint() {
		t.Fatal("fingerprint should change when a CallFact differs")
	}
}
