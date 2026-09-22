package main

import "testing"

func TestComputeParity(t *testing.T) {
	tests := []struct {
		name        string
		goFindings  []parityGoFinding
		diagnostics map[string][]compilerDiagnostic
		rule        string
		want        parityCount
	}{
		{"byte overlap", []parityGoFinding{{Rule: "UnsafeCast", RelPath: "a.kt", Line: 2, Col: 4, StartByte: 10, EndByte: 20, HasBytes: true}}, map[string][]compilerDiagnostic{"a.kt": {{FactoryName: "CAST_NEVER_SUCCEEDS", Line: 2, Col: 4, StartByte: 15, EndByte: 25}}}, "UnsafeCast", parityCount{Rule: "UnsafeCast", Agree: 1}},
		{"go only", []parityGoFinding{{Rule: "UnreachableCode", RelPath: "a.kt", Line: 2}}, nil, "UnreachableCode", parityCount{Rule: "UnreachableCode", GoOnly: 1}},
		{"compiler only", nil, map[string][]compilerDiagnostic{"a.kt": {{FactoryName: "USELESS_ELVIS", Line: 3}}}, "UselessElvisOnNonNull", parityCount{Rule: "UselessElvisOnNonNull", CompilerOnly: 1}},
		{"line fallback", []parityGoFinding{{Rule: "UnsafeCast", RelPath: "a.kt", Line: 5, Col: 1}}, map[string][]compilerDiagnostic{"a.kt": {{FactoryName: "CAST_NEVER_SUCCEEDS", Line: 5, Col: 99, StartByte: 1, EndByte: 2}}}, "UnsafeCast", parityCount{Rule: "UnsafeCast", Agree: 1}},
		{"unmapped ignored", []parityGoFinding{{Rule: "OtherRule", RelPath: "a.kt", Line: 1}}, map[string][]compilerDiagnostic{"a.kt": {{FactoryName: "OTHER_FACTORY", Line: 1}}}, "UnsafeCast", parityCount{Rule: "UnsafeCast"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counts, _ := computeParity(tt.goFindings, tt.diagnostics)
			if len(counts) != len(parityMapping) {
				t.Fatalf("count entries = %d, want %d", len(counts), len(parityMapping))
			}
			var got parityCount
			for _, count := range counts {
				if count.Rule == tt.rule {
					got = count
				}
			}
			if got != tt.want {
				t.Fatalf("count = %#v, want %#v", got, tt.want)
			}
		})
	}
}
