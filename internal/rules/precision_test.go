package rules

import "testing"

func TestRulePrecision(t *testing.T) {
	tests := []struct {
		name string
		want Precision
	}{
		{"OldTargetApi", PrecisionPolicy},
		{"MissingPermission", PrecisionHeuristicTextBacked},
		{"ArrayPrimitive", PrecisionHeuristicTextBacked},
		{"DoubleMutabilityForCollection", PrecisionTypeAware},
		{"AllowBackupManifest", PrecisionProjectStructure},
		{"MagicNumber", PrecisionHeuristicTextBacked},
		{"OptionalAbstractKeyword", PrecisionASTBacked},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var found Rule
			for _, r := range Registry {
				if r.Name() == tt.name {
					found = r
					break
				}
			}
			if found == nil {
				t.Fatalf("rule %q not found", tt.name)
			}
			if got := RulePrecision(found); got != tt.want {
				t.Fatalf("RulePrecision(%s) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
