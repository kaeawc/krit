package sysres

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeTree writes files (relative path -> content) under a temp dir.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCgroupMemoryLimit(t *testing.T) {
	const gib = uint64(1) << 30
	cases := []struct {
		name      string
		procCg    string
		tree      map[string]string
		wantLimit uint64
		wantOK    bool
	}{
		{
			name:      "v2 namespaced root",
			procCg:    "0::/\n",
			tree:      map[string]string{"memory.max": "8589934592\n"},
			wantLimit: 8 * gib, wantOK: true,
		},
		{
			name:   "v2 unlimited",
			procCg: "0::/\n",
			tree:   map[string]string{"memory.max": "max\n"},
		},
		{
			name:   "v2 leaf unlimited, parent limited",
			procCg: "0::/build/action\n",
			tree: map[string]string{
				"build/action/memory.max": "max\n",
				"build/memory.max":        "4294967296\n",
			},
			wantLimit: 4 * gib, wantOK: true,
		},
		{
			name:   "v2 takes the smallest limit up the tree",
			procCg: "0::/a/b\n",
			tree: map[string]string{
				"a/b/memory.max": "6442450944\n",
				"a/memory.max":   "17179869184\n",
			},
			wantLimit: 6 * gib, wantOK: true,
		},
		{
			name:      "v2 host path not visible falls back to the mount root",
			procCg:    "0::/system.slice/docker-abc.scope\n",
			tree:      map[string]string{"memory.max": "2147483648\n"},
			wantLimit: 2 * gib, wantOK: true,
		},
		{
			name:      "v1 memory controller",
			procCg:    "12:cpu,cpuacct:/docker/abc\n7:memory:/docker/abc\n",
			tree:      map[string]string{"memory/docker/abc/memory.limit_in_bytes": "3221225472\n"},
			wantLimit: 3 * gib, wantOK: true,
		},
		{
			name:   "v1 unlimited sentinel",
			procCg: "7:memory:/\n",
			tree:   map[string]string{"memory/memory.limit_in_bytes": "9223372036854771712\n"},
		},
		{
			name:      "hybrid v1 and v2 prefers the v2 limit",
			procCg:    "0::/\n7:memory:/\n",
			tree:      map[string]string{"memory.max": "1073741824\n", "memory/memory.limit_in_bytes": "2147483648\n"},
			wantLimit: gib, wantOK: true,
		},
		{
			name:      "hybrid with unlimited v2 uses v1",
			procCg:    "0::/\n7:memory:/\n",
			tree:      map[string]string{"memory.max": "max\n", "memory/memory.limit_in_bytes": "2147483648\n"},
			wantLimit: 2 * gib, wantOK: true,
		},
		{
			name:   "no cgroup files",
			procCg: "0::/\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := writeTree(t, c.tree)
			procFile := filepath.Join(t.TempDir(), "cgroup")
			if err := os.WriteFile(procFile, []byte(c.procCg), 0o644); err != nil {
				t.Fatal(err)
			}
			got, ok := cgroupMemoryLimit(procFile, root)
			if got != c.wantLimit || ok != c.wantOK {
				t.Errorf("cgroupMemoryLimit = (%d, %v), want (%d, %v)", got, ok, c.wantLimit, c.wantOK)
			}
		})
	}
}

func TestCgroupMemoryLimitMissingProcFile(t *testing.T) {
	if _, ok := cgroupMemoryLimit(filepath.Join(t.TempDir(), "absent"), t.TempDir()); ok {
		t.Fatal("expected no limit without /proc/self/cgroup")
	}
}

func TestMemTotalBytes(t *testing.T) {
	file := filepath.Join(t.TempDir(), "meminfo")
	if err := os.WriteFile(file, []byte("MemFree:  1024 kB\nMemTotal:       16384 kB\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := memTotalBytes(file), uint64(16384)<<10; got != want {
		t.Errorf("memTotalBytes = %d, want %d", got, want)
	}
	if got := memTotalBytes(filepath.Join(t.TempDir(), "absent")); got != 0 {
		t.Errorf("memTotalBytes(missing) = %d, want 0", got)
	}
}

func TestCPUsFollowsGOMAXPROCS(t *testing.T) {
	prev := runtime.GOMAXPROCS(2)
	t.Cleanup(func() { runtime.GOMAXPROCS(prev) })
	if got := CPUs(); got != 2 {
		t.Errorf("CPUs() = %d, want 2", got)
	}
}
