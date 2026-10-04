package oracle

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/buildid"
)

func TestClasspathChangesStoreScopeAndTypesFreshness(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "dep.jar")
	types := filepath.Join(dir, "types.json")
	if err := os.WriteFile(jar, []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(types, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	before := NewStoreScope(BackendFIR, "", []string{jar})
	if err := RecordTypesFacts(types, false, before); err != nil {
		t.Fatal(err)
	}
	if !TypesJSONSatisfies(types, false, before, buildid.Token()) {
		t.Fatal("fresh types rejected")
	}
	time.Sleep(time.Millisecond)
	if err := os.WriteFile(jar, []byte("replaced jar"), 0644); err != nil {
		t.Fatal(err)
	}
	after := NewStoreScope(BackendFIR, "", []string{jar})
	if before.version == after.version {
		t.Fatal("store version unchanged")
	}
	if TypesJSONSatisfies(types, false, after, buildid.Token()) {
		t.Fatal("stale types accepted")
	}
}

func TestJvmTargetChangesFIRStoreScope(t *testing.T) {
	a := NewStoreScopeWithTarget(BackendFIR, "", nil, "11")
	b := NewStoreScopeWithTarget(BackendFIR, "", nil, "17")
	if a.version == b.version {
		t.Fatal("JVM target did not affect FIR store scope")
	}
}
