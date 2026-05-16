package deltarisk

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/kaeawc/krit/internal/snapshot"
)

// captureStdout swaps os.Stdout for the duration of fn and returns
// what fn writes. Kept local so this package has no test-only
// dependency on other CLI packages.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()
	fn()
	_ = w.Close()
	<-done
	os.Stdout = orig
	return buf.String()
}

// seedTwoSnapshots writes two captured snapshots with diverging
// per-module metrics so the cli has a non-empty vector to print.
func seedTwoSnapshots(t *testing.T, repoRoot string) (fromSHA, toSHA string) {
	t.Helper()
	root := snapshot.SnapshotsDir(repoRoot)
	fromSHA = "1111111111111111111111111111111111111111"
	toSHA = "2222222222222222222222222222222222222222"

	for i, sha := range []string{fromSHA, toSHA} {
		blob := &snapshot.Blob{
			SchemaVersion: snapshot.SchemaVersion,
			CommitSHA:     sha,
			CapturedAt:    int64(i + 1),
			Modules:       []snapshot.Module{{Path: ":core"}},
			Files:         []snapshot.File{{Path: "core/A.kt", Module: ":core"}},
		}
		if _, err := snapshot.Save(root, blob); err != nil {
			t.Fatalf("Save %s: %v", sha, err)
		}
		mods := []snapshot.ModuleMetrics{{Path: ":core", LOC: 100, Cyclomatic: 10, FanIn: 1}}
		if i == 1 {
			mods[0].LOC = 200
			mods[0].Cyclomatic = 20
			mods[0].FanIn = 5
		}
		metrics := &snapshot.Metrics{
			SchemaVersion: snapshot.MetricsSchemaVersion,
			CommitSHA:     sha,
			CapturedAt:    int64(i + 1),
			Modules:       mods,
			Files:         []snapshot.FileMetrics{{Path: "core/A.kt", Module: ":core", LOC: mods[0].LOC, Cyclomatic: mods[0].Cyclomatic}},
		}
		if _, err := snapshot.SaveMetrics(root, metrics); err != nil {
			t.Fatalf("SaveMetrics %s: %v", sha, err)
		}
		man := &snapshot.Manifest{
			SchemaVersion: snapshot.ManifestSchemaVersion,
			CommitSHA:     sha,
			CapturedAt:    int64(i + 1),
			BlobSchema:    snapshot.SchemaVersion,
			MetricsSchema: snapshot.MetricsSchemaVersion,
			Files:         1, Modules: 1,
		}
		if _, err := snapshot.SaveManifest(root, man); err != nil {
			t.Fatalf("SaveManifest %s: %v", sha, err)
		}
	}
	return fromSHA, toSHA
}

func TestRunDeltaRiskJSON(t *testing.T) {
	repoRoot := t.TempDir()
	from, to := seedTwoSnapshots(t, repoRoot)

	out := captureStdout(t, func() {
		code := Run([]string{
			"--repo", repoRoot,
			"--from", from,
			"--to", to,
			"--format", "json",
		})
		if code != 0 {
			t.Fatalf("Run exit = %d", code)
		}
	})

	var res struct {
		Vector map[string]float64 `json:"vector"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decode: %v: %s", err, out)
	}
	if _, ok := res.Vector[":core::loc"]; !ok {
		t.Fatalf("vector missing :core::loc: %+v", res.Vector)
	}
	if got := res.Vector[":core::loc"]; got != 100 {
		t.Errorf(":core::loc = %g, want 100", got)
	}
}

func TestRunDeltaRiskRequiresBothSHAs(t *testing.T) {
	if code := Run([]string{"--from", "abc"}); code == 0 {
		t.Fatalf("missing --to should fail")
	}
	if code := Run([]string{"--to", "abc"}); code == 0 {
		t.Fatalf("missing --from should fail")
	}
}
