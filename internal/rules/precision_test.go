package rules

import (
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func TestRulePrecision(t *testing.T) {
	tests := []struct {
		name string
		want Precision
	}{
		{"OldTargetApi", PrecisionPolicy},
		{"MissingPermission", PrecisionTypeAware},
		{"ArrayPrimitive", PrecisionHeuristicTextBacked},
		{"DoubleMutabilityForCollection", PrecisionTypeAware},
		{"AllowBackupManifest", PrecisionProjectStructure},
		{"MagicNumber", PrecisionHeuristicTextBacked},
		{"OptionalAbstractKeyword", PrecisionASTBacked},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var found *v2.Rule
			for _, r := range v2.Registry {
				if r.ID == tt.name {
					found = r
					break
				}
			}
			if found == nil {
				t.Fatalf("rule %q not found", tt.name)
			}
			if got := V2RulePrecision(found); got != tt.want {
				t.Fatalf("V2RulePrecision(%s) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
