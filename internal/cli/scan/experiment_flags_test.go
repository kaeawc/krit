package scan

import (
	"reflect"
	"testing"
)

func TestPartitionDeprecatedExperiments(t *testing.T) {
	deprecated := func(s string) bool {
		return s == "old-1" || s == "old-2"
	}

	cases := []struct {
		name           string
		input          []string
		wantKept       []string
		wantDeprecated []string
	}{
		{
			name:           "empty input",
			input:          nil,
			wantKept:       nil,
			wantDeprecated: nil,
		},
		{
			name:           "all kept",
			input:          []string{"new-a", "new-b"},
			wantKept:       []string{"new-a", "new-b"},
			wantDeprecated: nil,
		},
		{
			name:           "all deprecated",
			input:          []string{"old-1", "old-2"},
			wantKept:       nil,
			wantDeprecated: []string{"old-1", "old-2"},
		},
		{
			name:           "mixed preserves input order",
			input:          []string{"new-a", "old-1", "new-b", "old-2"},
			wantKept:       []string{"new-a", "new-b"},
			wantDeprecated: []string{"old-1", "old-2"},
		},
		{
			name:           "duplicate deprecated reported each time",
			input:          []string{"old-1", "new-a", "old-1"},
			wantKept:       []string{"new-a"},
			wantDeprecated: []string{"old-1", "old-1"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotKept, gotDeprecated := partitionDeprecatedExperiments(tc.input, deprecated)
			if !reflect.DeepEqual(gotKept, tc.wantKept) {
				t.Errorf("kept = %v; want %v", gotKept, tc.wantKept)
			}
			if !reflect.DeepEqual(gotDeprecated, tc.wantDeprecated) {
				t.Errorf("deprecated = %v; want %v", gotDeprecated, tc.wantDeprecated)
			}
		})
	}
}
