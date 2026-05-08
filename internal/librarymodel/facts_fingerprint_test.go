package librarymodel

import "testing"

func TestFactsFingerprint_StableForEqualValues(t *testing.T) {
	a := DefaultFacts()
	b := DefaultFacts()
	if got, want := a.Fingerprint(), b.Fingerprint(); got != want {
		t.Fatalf("equal Facts produced different fingerprints:\n  a=%s\n  b=%s", got, want)
	}
}

func TestFactsFingerprint_NilDistinct(t *testing.T) {
	var nilFacts *Facts
	zero := &Facts{}
	full := DefaultFacts()
	a, b, c := nilFacts.Fingerprint(), zero.Fingerprint(), full.Fingerprint()
	if a == b {
		t.Errorf("nil Facts fingerprint matches zero Facts: %s", a)
	}
	if b == c {
		t.Errorf("zero Facts fingerprint matches default Facts: %s", b)
	}
}

func TestFactsFingerprint_SensitiveToProfile(t *testing.T) {
	base := DefaultFacts()
	tweaked := DefaultFacts()
	tweaked.Profile.Dependencies = append(tweaked.Profile.Dependencies, Dependency{
		Group:         "androidx.room",
		Name:          "room-runtime",
		Configuration: "implementation",
	})
	if base.Fingerprint() == tweaked.Fingerprint() {
		t.Fatal("expected fingerprint to change when a dependency is added")
	}
}

func TestFactsFingerprint_SensitiveToDatabaseFlag(t *testing.T) {
	base := DefaultFacts()
	tweaked := DefaultFacts()
	tweaked.Database.Room.Enabled = !tweaked.Database.Room.Enabled
	if base.Fingerprint() == tweaked.Fingerprint() {
		t.Fatal("expected fingerprint to change when Room.Enabled flips")
	}
}
