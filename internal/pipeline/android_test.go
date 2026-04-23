package pipeline

import (
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules"
)

func TestAndroidValuesScanKinds(t *testing.T) {
	tests := []struct {
		name string
		deps rules.AndroidDataDependency
		want android.ValuesScanKind
	}{
		{
			name: "none",
			deps: rules.AndroidDepNone,
			want: android.ValuesScanNone,
		},
		{
			name: "layout-only",
			deps: rules.AndroidDepLayout,
			want: android.ValuesScanNone,
		},
		{
			name: "strings",
			deps: rules.AndroidDepValuesStrings,
			want: android.ValuesScanStrings,
		},
		{
			name: "dimensions",
			deps: rules.AndroidDepValuesDimensions,
			want: android.ValuesScanDimensions,
		},
		{
			name: "plurals",
			deps: rules.AndroidDepValuesPlurals,
			want: android.ValuesScanPlurals,
		},
		{
			name: "arrays",
			deps: rules.AndroidDepValuesArrays,
			want: android.ValuesScanArrays,
		},
		{
			name: "extra-text",
			deps: rules.AndroidDepValuesExtraText,
			want: android.ValuesScanExtraText,
		},
		{
			name: "mixed",
			deps: rules.AndroidDepValuesStrings | rules.AndroidDepValuesPlurals,
			want: android.ValuesScanStrings | android.ValuesScanPlurals,
		},
		{
			name: "all-values",
			deps: rules.AndroidDepValues,
			want: android.ValuesScanAll,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := androidValuesScanKinds(tt.deps); got != tt.want {
				t.Fatalf("androidValuesScanKinds(%v) = %v, want %v", tt.deps, got, tt.want)
			}
		})
	}
}
