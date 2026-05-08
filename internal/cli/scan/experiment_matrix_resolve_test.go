package scan

import (
	"reflect"
	"testing"
)

func TestResolveExperimentCandidates(t *testing.T) {
	fallback := []string{"alpha", "beta", "gamma"}

	cases := []struct {
		name   string
		csv    []string
		intent []string
		want   []string
	}{
		{
			name:   "no csv, no intent uses fallback",
			csv:    nil,
			intent: nil,
			want:   fallback,
		},
		{
			name:   "csv only, no intent passes csv through",
			csv:    []string{"x", "y"},
			intent: nil,
			want:   []string{"x", "y"},
		},
		{
			name:   "no csv, intent uses intent set",
			csv:    nil,
			intent: []string{"foo", "bar"},
			want:   []string{"foo", "bar"},
		},
		{
			name:   "csv intersected with intent preserves csv order",
			csv:    []string{"c", "a", "b"},
			intent: []string{"a", "b"},
			want:   []string{"a", "b"},
		},
		{
			name:   "intersection order tracks csv not intent",
			csv:    []string{"b", "a"},
			intent: []string{"a", "b"},
			want:   []string{"b", "a"},
		},
		{
			name:   "zero intersection falls back to catalog",
			csv:    []string{"x"},
			intent: []string{"a", "b"},
			want:   fallback,
		},
		{
			name:   "empty csv with intent does not fall through to fallback",
			csv:    []string{},
			intent: []string{"only-intent"},
			want:   []string{"only-intent"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveExperimentCandidates(tc.csv, tc.intent, fallback)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("resolveExperimentCandidates(%v, %v, %v) = %v; want %v",
					tc.csv, tc.intent, fallback, got, tc.want)
			}
		})
	}
}

func TestResolveExperimentCandidatesEmptyFallback(t *testing.T) {
	// Verify the fallback can itself be empty without panicking — useful for
	// callers that don't have a sensible default and pass nil.
	got := resolveExperimentCandidates(nil, nil, nil)
	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}
