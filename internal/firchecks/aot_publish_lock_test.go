package firchecks

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/fslock"
)

func registryFilesPresent(key string) []string {
	var present []string
	for _, p := range []string{firPIDPath(key), firPortPath(key), firAOTPath(key)} {
		if _, err := os.Stat(p); err == nil {
			present = append(present, p)
		}
	}
	return present
}

// Common case: nobody else is publishing. The daemon is registered and a
// later run reuses it.
func TestUncontendedNonAOTDaemonPublishesAndIsReused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KRIT_DAEMON_REGISTRY_DIR", t.TempDir())
	pid, port := pingListener(t)
	d := &FirDaemon{port: port}
	if err := publishFirDaemon(d, pid, "plain", false); err != nil {
		t.Fatal(err)
	}
	if d.unpublished {
		t.Fatal("uncontended daemon was kept private")
	}
	re, err := connectExistingFirDaemon("plain", false)
	if err != nil {
		t.Fatal(err)
	}
	defer re.conn.Close()
	if !re.aotVerified || re.aotCachePath != "" {
		t.Fatalf("reconnected daemon verified=%v cache=%q, want reusable non-AOT", re.aotVerified, re.aotCachePath)
	}
}

// When another process holds the publication lock the registry is not
// written at all and the daemon stays private.
func TestFailedClaimLeavesRegistryUntouchedAndDaemonPrivate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KRIT_DAEMON_REGISTRY_DIR", t.TempDir())
	oldWait := firClaimWait
	firClaimWait = 50 * time.Millisecond
	t.Cleanup(func() { firClaimWait = oldWait })

	otherPID, otherPort := pingListener(t)
	must(t, writeFirPIDFile(otherPID, otherPort, "busy"))
	before := map[string]string{}
	for _, p := range registryFilesPresent("busy") {
		data, _ := os.ReadFile(p)
		before[p] = string(data)
	}

	holder, ok := fslock.Acquire(firLockPath("busy"), time.Second)
	if !ok {
		t.Fatal("could not take the lock to simulate a concurrent publisher")
	}
	defer holder()

	pid, port := pingListener(t)
	d := &FirDaemon{port: port}
	if err := publishFirDaemon(d, pid, "busy", true); err != nil {
		t.Fatalf("contended publish must keep the daemon private, got %v", err)
	}
	if !d.unpublished {
		t.Fatal("daemon not private after a failed claim")
	}
	if err := writeFirPIDFile(pid, port, "busy"); !errors.Is(err, errFirRegistryBusy) {
		t.Fatalf("writeFirPIDFile without the lock = %v, want errFirRegistryBusy", err)
	}
	for p, want := range before {
		got, err := os.ReadFile(p)
		if err != nil || string(got) != want {
			t.Fatalf("registry file %s changed: %q, %v", p, got, err)
		}
	}
	if len(registryFilesPresent("busy")) != len(before) {
		t.Fatalf("registry gained files: %v", registryFilesPresent("busy"))
	}

	// With no prior registration at all, a failed claim must not create one.
	freshHolder, ok := fslock.Acquire(firLockPath("fresh"), time.Second)
	if !ok {
		t.Fatal("could not lock the fresh key")
	}
	defer freshHolder()
	fresh := &FirDaemon{port: port}
	if err := publishFirDaemon(fresh, pid, "fresh", false); err != nil || !fresh.unpublished {
		t.Fatalf("fresh contended publish: err=%v unpublished=%v", err, fresh.unpublished)
	}
	if got := registryFilesPresent("fresh"); len(got) != 0 {
		t.Fatalf("failed claim created registry files: %v", got)
	}
}
