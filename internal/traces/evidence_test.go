package traces

import "testing"

// TestAdjustConfidenceDowngradesObservedSymbol: a symbol the static
// graph thinks is dead but traces show as live should get its
// confidence dampened, not eliminated.
func TestAdjustConfidenceDowngradesObservedSymbol(t *testing.T) {
	store := &Store{
		Sources: []IngestSource{{ID: "otel-1"}},
		States: []RuntimeState{
			{Fingerprint: "fp1", TopSymbol: "com.acme.LivelyFn"},
		},
	}
	if got := store.AdjustConfidence(0.9, "com.acme.LivelyFn", 0.3); got != 0.9*0.3 {
		t.Fatalf("observed symbol: want downgrade 0.27, got %v", got)
	}
	if got := store.AdjustConfidence(0.9, "com.acme.DeadFn", 0.3); got != 0.9 {
		t.Fatalf("unobserved symbol: want base 0.9, got %v", got)
	}
}

func TestAdjustConfidenceNilStoreReturnsBase(t *testing.T) {
	var store *Store
	if got := store.AdjustConfidence(0.9, "x", 0.3); got != 0.9 {
		t.Fatalf("nil store: want base 0.9, got %v", got)
	}
}

func TestAdjustConfidenceEmptyStoreReturnsBase(t *testing.T) {
	store := &Store{}
	if got := store.AdjustConfidence(0.9, "x", 0.3); got != 0.9 {
		t.Fatalf("empty store: want base 0.9, got %v", got)
	}
}
