package main

import (
	"testing"

	"github.com/kaeawc/krit/internal/pipeline"
)

func TestPhaseWorkerCount(t *testing.T) {
	tests := []struct {
		name     string
		phase    string
		max      int
		items    int
		expected int
	}{
		{name: "parse caps at items", phase: "parse", max: 16, items: 4, expected: 4},
		{name: "parse caps at 16", phase: "parse", max: 32, items: 100, expected: 16},
		{name: "rule execution caps at 16", phase: "ruleExecution", max: 24, items: 1000, expected: 16},
		{name: "cross file caps at 16", phase: "crossFileAnalysis", max: 20, items: 500, expected: 16},
		{name: "module caps at 8", phase: "moduleAwareAnalysis", max: 24, items: 40, expected: 8},
		{name: "module caps at items", phase: "moduleAwareAnalysis", max: 24, items: 3, expected: 3},
		{name: "unknown phase uses max", phase: "other", max: 12, items: 50, expected: 12},
		{name: "zero items uses one worker", phase: "parse", max: 16, items: 0, expected: 1},
		{name: "zero max uses one worker", phase: "parse", max: 0, items: 10, expected: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pipeline.PhaseWorkerCount(tt.phase, tt.max, tt.items); got != tt.expected {
				t.Fatalf("PhaseWorkerCount(%q, %d, %d) = %d, want %d", tt.phase, tt.max, tt.items, got, tt.expected)
			}
		})
	}
}
