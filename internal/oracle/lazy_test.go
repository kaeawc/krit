package oracle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeLazyOracleJSON(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "types.json")
	data := Data{
		Version: 1,
		Files:   map[string]*File{},
		Dependencies: map[string]*Class{
			"com.example.Foo": {
				FQN:        "com.example.Foo",
				Kind:       "class",
				Visibility: "public",
			},
		},
	}
	body, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal oracle data: %v", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write oracle data: %v", err)
	}
	return path
}

func TestLazyLookupDefersLoadUntilLookup(t *testing.T) {
	path := writeLazyOracleJSON(t, t.TempDir())
	lazy := NewLazyLookup(path, nil)

	if lazy.Loaded() {
		t.Fatal("new lazy lookup should not be loaded")
	}
	if got := lazy.Stats(); got != (Stats{}) {
		t.Fatalf("Stats should not force load, got %+v", got)
	}
	if lazy.Loaded() {
		t.Fatal("Stats forced oracle load")
	}

	info := lazy.LookupClass("Foo")
	if info == nil {
		t.Fatal("LookupClass returned nil")
	}
	if info.FQN != "com.example.Foo" {
		t.Fatalf("FQN = %q, want com.example.Foo", info.FQN)
	}
	if !lazy.Loaded() {
		t.Fatal("lookup should load oracle data")
	}
}

func TestLazyLookupReportsLoadErrorOnce(t *testing.T) {
	var reported int
	lazy := NewLazyLookup(filepath.Join(t.TempDir(), "missing.json"), func(error) {
		reported++
	})

	if got := lazy.LookupClass("Missing"); got != nil {
		t.Fatalf("LookupClass = %+v, want nil", got)
	}
	if got := lazy.LookupFunction("Missing.fn"); got != nil {
		t.Fatalf("LookupFunction = %+v, want nil", got)
	}
	if reported != 1 {
		t.Fatalf("load errors reported %d times, want 1", reported)
	}
	if lazy.Err() == nil {
		t.Fatal("expected retained load error")
	}
}
