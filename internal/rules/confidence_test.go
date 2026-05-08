package rules

import "testing"

func TestConfidenceTierConstants(t *testing.T) {
	if !(ConfidenceLow < ConfidenceMedium && ConfidenceMedium < ConfidenceHigh) {
		t.Fatalf("confidence tiers must be ordered low < medium < high, got low=%v medium=%v high=%v",
			ConfidenceLow, ConfidenceMedium, ConfidenceHigh)
	}
	if got := (ManifestBase{}).Confidence(); got != ConfidenceMedium {
		t.Fatalf("ManifestBase confidence = %v, want %v", got, ConfidenceMedium)
	}
}
