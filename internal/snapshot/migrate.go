package snapshot

import (
	"errors"
	"fmt"
)

// Schema bump checklist for contributors:
//
//  1. Bump the relevant *SchemaVersion constant by one (SchemaVersion,
//     MetricsSchemaVersion, ManifestSchemaVersion).
//  2. Add a migrator entry to the matching map below — keyed by the
//     SOURCE version, returning the value at version+1.
//  3. Add a fixture round-trip test under migrate_test.go: encode at
//     the old version, run Migrate*, assert the new shape.
//
// The migrators are intentionally small: each step climbs exactly one
// version. The driver chains them so a v1 → v3 jump runs the v1→v2
// and v2→v3 migrators in order, picking up any field defaults and
// schema-touching transformations along the way.

// blobMigrators is keyed by source schema. blobMigrators[N] returns
// the v(N+1) representation. Empty until the first real bump.
var blobMigrators = map[int]func(*Blob) (*Blob, error){}

// metricsMigrators is the per-step migration table for *Metrics.
var metricsMigrators = map[int]func(*Metrics) (*Metrics, error){}

// manifestMigrators is the per-step migration table for *Manifest.
var manifestMigrators = map[int]func(*Manifest) (*Manifest, error){}

// MigrateBlob walks blob's schema up to the current SchemaVersion.
// Returns blob unchanged when already current; refuses to operate on
// nil, on a future-schema blob (this binary is older than the data),
// or when the per-step migration table doesn't cover the gap.
func MigrateBlob(blob *Blob) (*Blob, error) {
	if blob == nil {
		return nil, errors.New("snapshot: MigrateBlob: nil blob")
	}
	cur := blob.SchemaVersion
	if err := checkMigrationTarget("blob", cur, SchemaVersion); err != nil {
		return nil, err
	}
	for cur < SchemaVersion {
		step, ok := blobMigrators[cur]
		if !ok {
			return nil, fmt.Errorf("snapshot: blob: no migrator from v%d to v%d", cur, cur+1)
		}
		next, err := step(blob)
		if err != nil {
			return nil, fmt.Errorf("snapshot: blob: migrate v%d -> v%d: %w", cur, cur+1, err)
		}
		if next == nil {
			return nil, fmt.Errorf("snapshot: blob: migrator v%d -> v%d returned nil", cur, cur+1)
		}
		if next.SchemaVersion != cur+1 {
			return nil, fmt.Errorf("snapshot: blob: migrator v%d -> v%d emitted SchemaVersion=%d", cur, cur+1, next.SchemaVersion)
		}
		blob = next
		cur = blob.SchemaVersion
	}
	return blob, nil
}

// MigrateMetrics is the *Metrics counterpart to MigrateBlob.
func MigrateMetrics(m *Metrics) (*Metrics, error) {
	if m == nil {
		return nil, errors.New("snapshot: MigrateMetrics: nil metrics")
	}
	cur := m.SchemaVersion
	if err := checkMigrationTarget("metrics", cur, MetricsSchemaVersion); err != nil {
		return nil, err
	}
	for cur < MetricsSchemaVersion {
		step, ok := metricsMigrators[cur]
		if !ok {
			return nil, fmt.Errorf("snapshot: metrics: no migrator from v%d to v%d", cur, cur+1)
		}
		next, err := step(m)
		if err != nil {
			return nil, fmt.Errorf("snapshot: metrics: migrate v%d -> v%d: %w", cur, cur+1, err)
		}
		if next == nil {
			return nil, fmt.Errorf("snapshot: metrics: migrator v%d -> v%d returned nil", cur, cur+1)
		}
		if next.SchemaVersion != cur+1 {
			return nil, fmt.Errorf("snapshot: metrics: migrator v%d -> v%d emitted SchemaVersion=%d", cur, cur+1, next.SchemaVersion)
		}
		m = next
		cur = m.SchemaVersion
	}
	return m, nil
}

// MigrateManifest is the *Manifest counterpart to MigrateBlob.
func MigrateManifest(m *Manifest) (*Manifest, error) {
	if m == nil {
		return nil, errors.New("snapshot: MigrateManifest: nil manifest")
	}
	cur := m.SchemaVersion
	if err := checkMigrationTarget("manifest", cur, ManifestSchemaVersion); err != nil {
		return nil, err
	}
	for cur < ManifestSchemaVersion {
		step, ok := manifestMigrators[cur]
		if !ok {
			return nil, fmt.Errorf("snapshot: manifest: no migrator from v%d to v%d", cur, cur+1)
		}
		next, err := step(m)
		if err != nil {
			return nil, fmt.Errorf("snapshot: manifest: migrate v%d -> v%d: %w", cur, cur+1, err)
		}
		if next == nil {
			return nil, fmt.Errorf("snapshot: manifest: migrator v%d -> v%d returned nil", cur, cur+1)
		}
		if next.SchemaVersion != cur+1 {
			return nil, fmt.Errorf("snapshot: manifest: migrator v%d -> v%d emitted SchemaVersion=%d", cur, cur+1, next.SchemaVersion)
		}
		m = next
		cur = m.SchemaVersion
	}
	return m, nil
}

// checkMigrationTarget enforces shared invariants: refuse zero / negative
// source versions (corrupted reads), refuse future versions (newer-than-
// binary data), and short-circuit when source already equals target.
func checkMigrationTarget(kind string, cur, target int) error {
	if cur <= 0 {
		return fmt.Errorf("snapshot: %s: invalid SchemaVersion %d", kind, cur)
	}
	if cur > target {
		return fmt.Errorf("snapshot: %s: schema v%d is newer than this binary's v%d (upgrade krit)", kind, cur, target)
	}
	return nil
}
