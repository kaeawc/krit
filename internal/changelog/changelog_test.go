package changelog

import (
	"strings"
	"testing"
)

func TestChangelogGrouping(t *testing.T) {
	snapshots := []Snapshot{
		{ID: "B", Category: "style", IntroducedIn: "0.2.0"},
		{ID: "A", Category: "perf", IntroducedIn: "0.2.0"},
		{ID: "C", Category: "style", IntroducedIn: "0.3.0", EnabledByDefaultSince: "0.4.0"},
		{ID: "D", Category: "android", IntroducedIn: "0.4.0"},
		{ID: "E", Category: "android", IntroducedIn: ""},
		{ID: "F", Category: "style", IntroducedIn: "0.10.0"},
	}

	got := GroupByVersion(snapshots)
	wantVersions := []string{"unreleased", "0.10.0", "0.4.0", "0.3.0", "0.2.0"}
	if len(got) != len(wantVersions) {
		t.Fatalf("Group returned %d groups, want %d (%v)", len(got), len(wantVersions), got)
	}
	for i, want := range wantVersions {
		if got[i].Version != want {
			t.Errorf("group[%d].Version = %q, want %q", i, got[i].Version, want)
		}
	}

	v020 := got[len(got)-1]
	if v020.Version != "0.2.0" {
		t.Fatalf("oldest group should be 0.2.0, got %q", v020.Version)
	}
	if len(v020.Entries) != 2 {
		t.Fatalf("0.2.0 should have 2 entries, got %d", len(v020.Entries))
	}
	// Within a version, entries are ordered by category then ID:
	// perf < style alphabetically.
	if v020.Entries[0].ID != "A" || v020.Entries[1].ID != "B" {
		t.Errorf("0.2.0 entries out of order: %+v", v020.Entries)
	}
}

func TestChangelogGroupingComparesNumericVersions(t *testing.T) {
	// 0.10.0 must sort newer than 0.9.0 — purely lexical sort would
	// reverse them.
	snapshots := []Snapshot{
		{ID: "Old", IntroducedIn: "0.9.0"},
		{ID: "New", IntroducedIn: "0.10.0"},
	}
	got := GroupByVersion(snapshots)
	if got[0].Version != "0.10.0" {
		t.Errorf("0.10.0 should sort newest, got %q first", got[0].Version)
	}
	if got[1].Version != "0.9.0" {
		t.Errorf("0.9.0 should sort after 0.10.0, got %q second", got[1].Version)
	}
}

func TestRenderProducesGroupedMarkdown(t *testing.T) {
	groups := []Group{
		{
			Version: "0.4.0",
			Entries: []Entry{
				{ID: "NewCheck", Category: "android", Description: "new check"},
			},
		},
		{
			Version: "0.2.0",
			Entries: []Entry{
				{ID: "Old", Category: "style", Description: "old check", EnabledByDefaultSince: "0.3.0"},
			},
		},
	}
	md := Render(groups, 0)
	if !strings.Contains(md, "## v0.4.0") {
		t.Errorf("rendered output missing v0.4.0 header:\n%s", md)
	}
	if !strings.Contains(md, "## v0.2.0") {
		t.Errorf("rendered output missing v0.2.0 header:\n%s", md)
	}
	if !strings.Contains(md, "default since 0.3.0") {
		t.Errorf("Old rule should advertise default since 0.3.0:\n%s", md)
	}
	if !strings.Contains(md, "**NewCheck**") {
		t.Errorf("rendered output missing NewCheck:\n%s", md)
	}
}

func TestRenderRespectsVersionLimit(t *testing.T) {
	groups := []Group{
		{Version: "0.4.0", Entries: []Entry{{ID: "A"}}},
		{Version: "0.3.0", Entries: []Entry{{ID: "B"}}},
		{Version: "0.2.0", Entries: []Entry{{ID: "C"}}},
	}
	md := Render(groups, 2)
	if !strings.Contains(md, "0.4.0") || !strings.Contains(md, "0.3.0") {
		t.Errorf("limit=2 should keep 0.4.0 and 0.3.0:\n%s", md)
	}
	if strings.Contains(md, "0.2.0") {
		t.Errorf("limit=2 should drop 0.2.0:\n%s", md)
	}
}
