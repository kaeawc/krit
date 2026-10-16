package main

import (
	"encoding/json"
	"testing"
)

func TestOracleBackendFindsNestedEntry(t *testing.T) {
	var data map[string]any
	report := `{"perfTiming":[{"name":"parse"},{"name":"indexPhaseRun","children":[{"name":"typeOracle","children":[
		{"name":"jvmAnalyze","children":[{"name":"oracleBackend","attributes":{"backend":"kaa","jar":"krit-types.jar"}}]}]}]}]}`
	if err := json.Unmarshal([]byte(report), &data); err != nil {
		t.Fatal(err)
	}
	if got := oracleBackend(data); got != "kaa" {
		t.Fatalf("oracleBackend = %q, want kaa", got)
	}
	if got := oracleBackend(map[string]any{}); got != "" {
		t.Fatalf("oracleBackend without an oracle = %q", got)
	}
}
