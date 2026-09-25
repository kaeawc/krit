package pipeline

import (
	"errors"
	"testing"
)

func TestShouldFallbackToAnalyzeAll(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		pruned bool
		want   bool
	}{
		{name: "clean merge", want: false},
		{name: "pruned cached files", pruned: true, want: true},
		{name: "merge error", err: errors.New("merge failed"), want: true},
		{name: "error and pruning", err: errors.New("merge failed"), pruned: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldFallbackToAnalyzeAll(tt.err, tt.pruned); got != tt.want {
				t.Fatalf("shouldFallbackToAnalyzeAll(%v, %v) = %v, want %v", tt.err, tt.pruned, got, tt.want)
			}
		})
	}
}
