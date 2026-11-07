package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompareIgnoresOrderAndNormalizesPaths(t *testing.T) {
	a := []Finding{
		{File: "/repo/m/A.kt", Line: 3, Rule: "R1", Message: "bad  thing"},
		{File: "/repo/m/B.kt", Line: 1, Rule: "R2", Message: "x"},
	}
	b := []Finding{
		{File: "/repo/m/B.kt", Line: 1, Rule: "R2", Message: "x"},
		{File: "/repo/m/A.kt", Line: 3, Rule: "R1", Message: "bad thing"},
	}
	d := Compare(Normalize(a, "/repo"), Normalize(b, "/repo"))
	if d.Total() != 0 || d.Matching != 2 {
		t.Fatalf("diff = %+v", d)
	}
}

func TestCompareCountsDuplicatesAndGroupsByRule(t *testing.T) {
	f := Finding{File: "A.kt", Line: 1, Rule: "R1", Message: "m"}
	g := Finding{File: "B.kt", Line: 2, Rule: "R2", Message: "n"}
	d := Compare([]Finding{f, g}, []Finding{f, f})
	if len(d.OnlyA["R2"]) != 1 || len(d.OnlyB["R1"]) != 1 || d.Matching != 1 {
		t.Fatalf("diff = %+v", d)
	}
	var out bytes.Buffer
	Report(&out, d, "whole", "module", 3, false)
	for _, want := range []string{"whole-only", "module-only", "R1", "R2", "TOTAL"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report missing %q:\n%s", want, out.String())
		}
	}
}

func TestNormalizeKeepsPathsOutsideRoot(t *testing.T) {
	got := Normalize([]Finding{{File: "/other/A.kt"}}, "/repo")
	if got[0].File != "/other/A.kt" {
		t.Fatalf("file = %q", got[0].File)
	}
}
