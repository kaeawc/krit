package snapshot

import (
	"strings"
	"testing"
)

func TestMigrateBlob_IdentityAtCurrentVersion(t *testing.T) {
	in := &Blob{SchemaVersion: SchemaVersion, CommitSHA: "abc"}
	got, err := MigrateBlob(in)
	if err != nil {
		t.Fatalf("MigrateBlob: %v", err)
	}
	if got != in {
		t.Errorf("expected the same pointer back at current version; got %p / %p", got, in)
	}
}

func TestMigrateBlob_RejectsNil(t *testing.T) {
	if _, err := MigrateBlob(nil); err == nil {
		t.Fatal("expected error on nil blob")
	}
}

func TestMigrateBlob_RejectsFutureSchema(t *testing.T) {
	in := &Blob{SchemaVersion: SchemaVersion + 1, CommitSHA: "abc"}
	_, err := MigrateBlob(in)
	if err == nil {
		t.Fatal("expected error on future schema")
	}
	if !strings.Contains(err.Error(), "newer than this binary") {
		t.Errorf("error should hint at upgrading: %v", err)
	}
}

func TestMigrateBlob_RejectsZeroSchema(t *testing.T) {
	in := &Blob{SchemaVersion: 0, CommitSHA: "abc"}
	if _, err := MigrateBlob(in); err == nil {
		t.Fatal("expected error on zero schema")
	}
}

func TestMigrateBlob_StepwiseMigratorChain(t *testing.T) {
	// Drive the loop body without touching production migrators by
	// installing a fake bridge. Skip when SchemaVersion is at v1
	// (no headroom to install a fake source).
	if SchemaVersion < 2 {
		t.Skip("no future schema to test against; revisit when SchemaVersion >= 2")
	}
	// (Placeholder for when SchemaVersion bumps; the existing
	// no-migrator-error path covers the negative case until then.)
}

func TestMigrateBlob_ErrorWhenStepMissing(t *testing.T) {
	// At schema v1 with no migrators registered, asking for a v0
	// blob should produce a clear "no migrator" error, not a panic.
	if SchemaVersion < 1 {
		t.Skip("nothing to migrate from")
	}
	// Construct a blob at v0 (intentionally invalid for production
	// data; allowed in the test to exercise the gap-detection path).
	// Drop the zero-version check by claiming v(SchemaVersion-1)+0 — we
	// need an unmigrated source. checkMigrationTarget rejects 0 so use
	// a synthetic one-version-back source instead, which only exists
	// when SchemaVersion >= 2.
	if SchemaVersion < 2 {
		t.Skip("no SchemaVersion gap to test missing-migrator behaviour")
	}
	in := &Blob{SchemaVersion: SchemaVersion - 1, CommitSHA: "abc"}
	_, err := MigrateBlob(in)
	if err == nil {
		t.Fatal("expected error when no migrator covers the gap")
	}
	if !strings.Contains(err.Error(), "no migrator") {
		t.Errorf("expected 'no migrator' message; got %v", err)
	}
}

func TestMigrateMetrics_IdentityAtCurrentVersion(t *testing.T) {
	in := &Metrics{SchemaVersion: MetricsSchemaVersion, CommitSHA: "abc"}
	got, err := MigrateMetrics(in)
	if err != nil {
		t.Fatalf("MigrateMetrics: %v", err)
	}
	if got != in {
		t.Errorf("expected pointer-equality at current version; got %p / %p", got, in)
	}
}

func TestMigrateMetrics_RejectsNil(t *testing.T) {
	if _, err := MigrateMetrics(nil); err == nil {
		t.Fatal("expected error on nil metrics")
	}
}

func TestMigrateMetrics_RejectsFuture(t *testing.T) {
	in := &Metrics{SchemaVersion: MetricsSchemaVersion + 1}
	if _, err := MigrateMetrics(in); err == nil {
		t.Fatal("expected error on future metrics schema")
	}
}

func TestMigrateManifest_IdentityAtCurrentVersion(t *testing.T) {
	in := &Manifest{SchemaVersion: ManifestSchemaVersion, CommitSHA: "abc"}
	got, err := MigrateManifest(in)
	if err != nil {
		t.Fatalf("MigrateManifest: %v", err)
	}
	if got != in {
		t.Errorf("expected pointer-equality at current version; got %p / %p", got, in)
	}
}

func TestMigrateManifest_RejectsNil(t *testing.T) {
	if _, err := MigrateManifest(nil); err == nil {
		t.Fatal("expected error on nil manifest")
	}
}

func TestMigrateManifest_RejectsFuture(t *testing.T) {
	in := &Manifest{SchemaVersion: ManifestSchemaVersion + 1}
	if _, err := MigrateManifest(in); err == nil {
		t.Fatal("expected error on future manifest schema")
	}
}

// TestMigratorChainExecutesInOrder uses a temporary migrator
// installation against the manifest table to prove the driver runs
// step migrators in version order. Restores the table on teardown so
// the rest of the test suite isn't affected.
func TestMigratorChainExecutesInOrder(t *testing.T) {
	// Capture and restore the current map so this test is hermetic.
	original := manifestMigrators
	t.Cleanup(func() { manifestMigrators = original })

	calls := []int{}
	manifestMigrators = map[int]func(*Manifest) (*Manifest, error){
		ManifestSchemaVersion - 2: func(m *Manifest) (*Manifest, error) {
			calls = append(calls, m.SchemaVersion)
			cp := *m
			cp.SchemaVersion = m.SchemaVersion + 1
			return &cp, nil
		},
		ManifestSchemaVersion - 1: func(m *Manifest) (*Manifest, error) {
			calls = append(calls, m.SchemaVersion)
			cp := *m
			cp.SchemaVersion = m.SchemaVersion + 1
			return &cp, nil
		},
	}
	if ManifestSchemaVersion < 3 {
		t.Skip("need ManifestSchemaVersion >= 3 to chain two steps; revisit on next bump")
	}

	in := &Manifest{SchemaVersion: ManifestSchemaVersion - 2, CommitSHA: "abc"}
	got, err := MigrateManifest(in)
	if err != nil {
		t.Fatalf("MigrateManifest: %v", err)
	}
	if got.SchemaVersion != ManifestSchemaVersion {
		t.Errorf("got %d; want %d", got.SchemaVersion, ManifestSchemaVersion)
	}
	if len(calls) != 2 || calls[0] != ManifestSchemaVersion-2 || calls[1] != ManifestSchemaVersion-1 {
		t.Errorf("expected ordered v(N-2) -> v(N-1) -> v(N) chain; got calls=%v", calls)
	}
}
