package oracle

import "testing"

func TestFullDeclarationProfile_IsFull(t *testing.T) {
	if !FullDeclarationProfile().IsFull() {
		t.Fatalf("FullDeclarationProfile should report IsFull() == true")
	}
	if FullDeclarationProfile().CLIValue() != "" {
		t.Fatalf("Full profile CLIValue must be empty so the flag is omitted")
	}
}

func TestFinalizeDeclarationProfile_FullHasEmptyFingerprint(t *testing.T) {
	summary := FinalizeDeclarationProfile(FullDeclarationProfile())
	if summary.Fingerprint != "" {
		t.Fatalf("full profile must fingerprint to empty (broad superset), got %q", summary.Fingerprint)
	}
}

func TestFinalizeDeclarationProfile_NarrowStable(t *testing.T) {
	narrow := DeclarationProfile{ClassShell: true, Supertypes: true, SourceDependencyClosure: true}
	a := FinalizeDeclarationProfile(narrow)
	b := FinalizeDeclarationProfile(narrow)
	if a.Fingerprint == "" {
		t.Fatalf("narrow profile must have a non-empty fingerprint")
	}
	if a.Fingerprint != b.Fingerprint {
		t.Fatalf("fingerprint must be stable across calls: %q vs %q", a.Fingerprint, b.Fingerprint)
	}
}

func TestFinalizeDeclarationProfile_DifferentProfilesDiffer(t *testing.T) {
	narrowA := DeclarationProfile{ClassShell: true, Supertypes: true}
	narrowB := DeclarationProfile{ClassShell: true, Supertypes: true, Members: true}
	a := FinalizeDeclarationProfile(narrowA).Fingerprint
	b := FinalizeDeclarationProfile(narrowB).Fingerprint
	if a == b {
		t.Fatalf("distinct profiles must produce distinct fingerprints, both %q", a)
	}
}

func TestParseDeclarationProfile_RoundTripsCLIValue(t *testing.T) {
	profile := DeclarationProfile{ClassShell: true, Supertypes: true, Members: true, SourceDependencyClosure: true}
	cli := profile.CLIValue()
	if cli == "" {
		t.Fatalf("narrow profile must yield a non-empty CLI value")
	}
	parsed := ParseDeclarationProfile(cli)
	if parsed != profile {
		t.Fatalf("round trip broken: want %+v got %+v (via %q)", profile, parsed, cli)
	}
}

func TestParseDeclarationProfile_IgnoresUnknown(t *testing.T) {
	got := ParseDeclarationProfile("classShell, bogus,members")
	want := DeclarationProfile{ClassShell: true, Members: true}
	if got != want {
		t.Fatalf("unknown names must be ignored; got %+v want %+v", got, want)
	}
}

func TestParseDeclarationProfile_EmptyYieldsZero(t *testing.T) {
	if (ParseDeclarationProfile("") != DeclarationProfile{}) {
		t.Fatalf("empty CLI value must yield the zero profile")
	}
	if (ParseDeclarationProfile("   ") != DeclarationProfile{}) {
		t.Fatalf("whitespace-only CLI value must yield the zero profile")
	}
}

func TestMergeDeclarationProfiles_TakesUnion(t *testing.T) {
	a := DeclarationProfile{ClassShell: true, Supertypes: true}
	b := DeclarationProfile{Members: true, MemberSignatures: true}
	merged := MergeDeclarationProfiles(a, b)
	want := DeclarationProfile{ClassShell: true, Supertypes: true, Members: true, MemberSignatures: true}
	if merged != want {
		t.Fatalf("merge must be the union; got %+v want %+v", merged, want)
	}
}

func TestCacheScopeCompatibleV2_DeclarationProfile(t *testing.T) {
	broadEntry := &CacheEntry{}
	narrowEntry := &CacheEntry{DeclarationProfileFingerprint: "abcd1234"}

	// Broad entry satisfies any lookup.
	if !cacheScopeCompatibleV2(broadEntry, "", "") {
		t.Fatalf("broad entry must satisfy broad lookup")
	}
	if !cacheScopeCompatibleV2(broadEntry, "", "abcd1234") {
		t.Fatalf("broad (full-profile) entry must satisfy a narrow lookup")
	}

	// Narrow entry only satisfies identical narrow lookup.
	if cacheScopeCompatibleV2(narrowEntry, "", "") {
		t.Fatalf("narrow entry must NOT satisfy a full-profile lookup")
	}
	if !cacheScopeCompatibleV2(narrowEntry, "", "abcd1234") {
		t.Fatalf("narrow entry must satisfy identical-fingerprint lookup")
	}
	if cacheScopeCompatibleV2(narrowEntry, "", "deadbeef") {
		t.Fatalf("narrow entry must NOT satisfy mismatched-fingerprint lookup")
	}
}
