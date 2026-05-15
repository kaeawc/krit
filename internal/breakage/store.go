package breakage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/kaeawc/krit/internal/fsutil"
)

const eventsFileName = "breakage_events.json"

// EventsFile is the on-disk shape of breakage_events.json.
type EventsFile struct {
	SchemaVersion int     `json:"schema_version"`
	Events        []Event `json:"events"`
}

// EventsPath returns the canonical events file inside a snapshots root.
func EventsPath(snapshotsRoot string) string {
	return filepath.Join(snapshotsRoot, eventsFileName)
}

var storeMu sync.Mutex

// Load reads the events file. Returns (nil, nil) when the file does
// not exist or carries a higher schema version than this binary
// understands so callers can degrade to "no historical events" without
// breaking.
func Load(snapshotsRoot string) (*EventsFile, error) {
	data, err := os.ReadFile(EventsPath(snapshotsRoot))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("breakage: read events: %w", err)
	}
	var ef EventsFile
	if err := json.Unmarshal(data, &ef); err != nil {
		return nil, fmt.Errorf("breakage: parse events: %w", err)
	}
	if ef.SchemaVersion > EventsSchemaVersion {
		return nil, nil
	}
	return &ef, nil
}

// LoadAll is a convenience that returns the events slice (possibly nil).
func LoadAll(snapshotsRoot string) ([]Event, error) {
	ef, err := Load(snapshotsRoot)
	if err != nil {
		return nil, err
	}
	if ef == nil {
		return nil, nil
	}
	return ef.Events, nil
}

// Record appends events to the on-disk file, deduplicating on Event.ID.
// Missing IDs are filled in from HashID. Validation errors abort the
// batch without writing — the returned count is 0 in that case.
func Record(snapshotsRoot string, events ...Event) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	storeMu.Lock()
	defer storeMu.Unlock()

	existing, err := Load(snapshotsRoot)
	if err != nil {
		return 0, err
	}
	ef := EventsFile{SchemaVersion: EventsSchemaVersion}
	if existing != nil {
		ef.Events = existing.Events
	}
	seen := make(map[string]struct{}, len(ef.Events))
	for _, e := range ef.Events {
		seen[e.ID] = struct{}{}
	}
	added := 0
	for _, e := range events {
		if e.CommitSHA == "" || e.FailureKind == "" {
			return 0, fmt.Errorf("breakage: event requires commit_sha and failure_kind")
		}
		if e.Source == "" {
			e.Source = SourceLocal
		}
		if e.Signature == "" {
			e.Signature = Normalize(e.Message)
		}
		if e.ID == "" {
			e.ID = HashID(e.FailureKind, e.Signature, e.CommitSHA, e.Source)
		}
		if _, dup := seen[e.ID]; dup {
			continue
		}
		seen[e.ID] = struct{}{}
		ef.Events = append(ef.Events, e)
		added++
	}
	if added == 0 {
		return 0, nil
	}
	sort.Slice(ef.Events, func(i, j int) bool {
		if ef.Events[i].OccurredAt != ef.Events[j].OccurredAt {
			return ef.Events[i].OccurredAt < ef.Events[j].OccurredAt
		}
		return ef.Events[i].ID < ef.Events[j].ID
	})

	payload, err := json.MarshalIndent(ef, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("breakage: marshal events: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.MkdirAll(snapshotsRoot, 0o755); err != nil {
		return 0, fmt.Errorf("breakage: mkdir %s: %w", snapshotsRoot, err)
	}
	if err := fsutil.WriteFileAtomic(EventsPath(snapshotsRoot), payload, 0o644); err != nil {
		return 0, fmt.Errorf("breakage: write events: %w", err)
	}
	return added, nil
}

// FindByID returns the event with the given ID, or (nil, nil) when not
// present.
func FindByID(snapshotsRoot, id string) (*Event, error) {
	events, err := LoadAll(snapshotsRoot)
	if err != nil {
		return nil, err
	}
	for i := range events {
		if events[i].ID == id {
			cp := events[i]
			return &cp, nil
		}
	}
	return nil, nil
}
