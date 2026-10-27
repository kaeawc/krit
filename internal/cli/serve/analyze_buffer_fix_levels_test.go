package serve

import (
	"encoding/json"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/scanner"
)

const fixLevelsBuffer = "package com.example\n\n" +
	"class Foo {\n" +
	"    fun bar() {\n" +
	"        val greeting = \"hello\".orEmpty()\n" +
	"        val transform = { it -> it + 1 }\n" +
	"        println(greeting + transform(1))\n" +
	"    }\n" +
	"}\n"

// Buffer results carry the fix level the CLI JSON reports for each rule
// with an autofix (the columns carry the fix but not its level), and only
// for rules whose findings have one.
func TestAnalyzeBuffer_FixLevels(t *testing.T) {
	socket, _ := startServerForTest(t)

	var got daemon.AnalyzeBufferResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeBuffer,
		daemon.AnalyzeBufferArgs{Path: "Foo.kt", Content: fixLevelsBuffer}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	assertFixLevels(t, got.Findings, got.FixLevels)

	var batch daemon.AnalyzeBuffersResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeBuffers, daemon.AnalyzeBuffersArgs{
		Buffers: []daemon.AnalyzeBufferArgs{{Path: "Foo.kt", Content: fixLevelsBuffer}},
	}, &batch); err != nil {
		t.Fatalf("batch call: %v", err)
	}
	if len(batch.Results) != 1 {
		t.Fatalf("batch results = %d, want 1", len(batch.Results))
	}
	assertFixLevels(t, batch.Results[0].Findings, batch.Results[0].FixLevels)
}

func assertFixLevels(t *testing.T, raw json.RawMessage, levels map[string]string) {
	t.Helper()
	var cols scanner.FindingColumns
	if err := json.Unmarshal(raw, &cols); err != nil {
		t.Fatalf("decode findings: %v", err)
	}
	want := map[string]string{
		"UselessCallOnNotNull":      "idiomatic",
		"ExplicitItLambdaParameter": "cosmetic",
	}
	for rule, level := range want {
		if levels[rule] != level {
			t.Errorf("fix_levels[%s] = %q, want %q (all: %v)", rule, levels[rule], level, levels)
		}
	}
	for row := 0; row < cols.Len(); row++ {
		rule := cols.RuleAt(row)
		if _, ok := levels[rule]; ok != cols.HasFix(row) {
			t.Errorf("rule %s: in fix_levels = %v, has fix = %v", rule, ok, cols.HasFix(row))
		}
	}
}
