package migration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMapSelectsMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "retrofit.yml")
	if err := os.WriteFile(path, []byte(`
library: retrofit
migrations:
  - from: 2.9.0
    to: 2.10.0
    symbols:
      - symbol: retrofit2.adapter.rxjava2.RxJava2CallAdapterFactory.create
        replacement: retrofit2.adapter.rxjava3.RxJava3CallAdapterFactory.create
        reason: use RxJava3 adapter
`), 0644); err != nil {
		t.Fatal(err)
	}
	migrationMap, err := LoadMap(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := migrationMap.Select("retrofit", "2.9.0", "2.10.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Replacement == "" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestMapValidateRequiresEntries(t *testing.T) {
	err := Map{Library: "retrofit", Migrations: []Migration{{From: "1", To: "2"}}}.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
}
