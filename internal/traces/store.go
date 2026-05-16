package traces

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"

	"github.com/kaeawc/krit/internal/fsutil"
)

// StoreDir is the on-disk directory under the repo root holding the
// traces store. Kept as a sibling of `.krit/snapshots` so that
// snapshot capture and trace ingest can ship independently.
const StoreDir = ".krit/traces"

const storeFileName = "store.json"

// Store is the in-memory representation of the persisted traces
// data. Loaded once per CLI invocation, mutated by ingest/reconcile,
// rewritten atomically by Save.
type Store struct {
	SchemaVersion int                   `json:"schema_version"`
	Sources       []IngestSource        `json:"sources,omitempty"`
	States        []RuntimeState        `json:"states,omitempty"`
	Transitions   []RuntimeTransition   `json:"transitions,omitempty"`
	Suggestions   []StateSuggestion     `json:"suggestions,omitempty"`
	Resolutions   map[string]Resolution `json:"resolutions,omitempty"`
}

// storeMu serialises read-modify-writes within a single process so
// concurrent ingest workers don't lose each other's appends.
var storeMu sync.Mutex

// StorePath returns the canonical store location under root.
func StorePath(root string) string {
	return filepath.Join(root, StoreDir, storeFileName)
}

// Load reads the persisted store. Missing files are not fatal: an
// empty store is returned so first-ingest can proceed without
// special-casing.
func Load(root string) (*Store, error) {
	path := StorePath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Store{SchemaVersion: SchemaVersion, Resolutions: map[string]Resolution{}}, nil
		}
		return nil, fmt.Errorf("traces: read store: %w", err)
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("traces: parse store: %w", err)
	}
	if s.SchemaVersion > SchemaVersion {
		return nil, fmt.Errorf("traces: store schema %d newer than supported %d", s.SchemaVersion, SchemaVersion)
	}
	if s.Resolutions == nil {
		s.Resolutions = map[string]Resolution{}
	}
	return &s, nil
}

// Save serialises the store as JSON and writes it atomically. The
// in-process mutex serialises concurrent saves from the same
// process — typically one ingest worker plus one reconcile run.
func (s *Store) Save(root string) error {
	storeMu.Lock()
	defer storeMu.Unlock()
	if s.SchemaVersion == 0 {
		s.SchemaVersion = SchemaVersion
	}
	// Sort for deterministic output.
	sort.Slice(s.Sources, func(i, j int) bool { return s.Sources[i].ID < s.Sources[j].ID })
	sort.Slice(s.States, func(i, j int) bool { return s.States[i].Fingerprint < s.States[j].Fingerprint })
	sort.Slice(s.Transitions, func(i, j int) bool {
		if s.Transitions[i].FromFP != s.Transitions[j].FromFP {
			return s.Transitions[i].FromFP < s.Transitions[j].FromFP
		}
		if s.Transitions[i].ToFP != s.Transitions[j].ToFP {
			return s.Transitions[i].ToFP < s.Transitions[j].ToFP
		}
		return s.Transitions[i].Kind < s.Transitions[j].Kind
	})
	sort.Slice(s.Suggestions, func(i, j int) bool {
		if s.Suggestions[i].Fingerprint != s.Suggestions[j].Fingerprint {
			return s.Suggestions[i].Fingerprint < s.Suggestions[j].Fingerprint
		}
		return s.Suggestions[i].Confidence > s.Suggestions[j].Confidence
	})
	payload, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("traces: marshal store: %w", err)
	}
	payload = append(payload, '\n')
	dir := filepath.Join(root, StoreDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("traces: mkdir %s: %w", dir, err)
	}
	if err := fsutil.WriteFileAtomic(StorePath(root), payload, 0o644); err != nil {
		return fmt.Errorf("traces: write store: %w", err)
	}
	return nil
}

// Merge folds a fresh set of states/transitions (typically from a
// single ingest run) into the store. Existing rows are aggregated:
// counts add, FirstSeen narrows, LastSeen widens, source IDs unique-
// merge.
func (s *Store) Merge(source IngestSource, states []RuntimeState, transitions []RuntimeTransition) {
	s.upsertSource(source)
	s.mergeStates(states)
	s.mergeTransitions(transitions)
}

func (s *Store) upsertSource(source IngestSource) {
	if source.ID == "" {
		return
	}
	for i := range s.Sources {
		if s.Sources[i].ID == source.ID {
			s.Sources[i] = source
			return
		}
	}
	s.Sources = append(s.Sources, source)
}

func (s *Store) mergeStates(states []RuntimeState) {
	byFP := make(map[string]int, len(s.States))
	for i := range s.States {
		byFP[s.States[i].Fingerprint] = i
	}
	for _, ns := range states {
		idx, ok := byFP[ns.Fingerprint]
		if !ok {
			s.States = append(s.States, ns)
			byFP[ns.Fingerprint] = len(s.States) - 1
			continue
		}
		existing := &s.States[idx]
		existing.Count += ns.Count
		mergeTimestamps(&existing.FirstSeen, &existing.LastSeen, ns.FirstSeen, ns.LastSeen)
		existing.Sources = unionStrings(existing.Sources, ns.Sources)
	}
}

func (s *Store) mergeTransitions(transitions []RuntimeTransition) {
	transKey := func(t RuntimeTransition) string {
		return t.FromFP + "|" + t.ToFP + "|" + string(t.Kind)
	}
	byTrans := make(map[string]int, len(s.Transitions))
	for i := range s.Transitions {
		byTrans[transKey(s.Transitions[i])] = i
	}
	for _, nt := range transitions {
		k := transKey(nt)
		idx, ok := byTrans[k]
		if !ok {
			s.Transitions = append(s.Transitions, nt)
			byTrans[k] = len(s.Transitions) - 1
			continue
		}
		existing := &s.Transitions[idx]
		existing.Count += nt.Count
		mergeTimestamps(&existing.FirstSeen, &existing.LastSeen, nt.FirstSeen, nt.LastSeen)
		existing.Sources = unionStrings(existing.Sources, nt.Sources)
	}
}

// mergeTimestamps narrows first toward incoming first (when non-zero)
// and widens last toward incoming last.
func mergeTimestamps(first, last *int64, incomingFirst, incomingLast int64) {
	if *first == 0 || (incomingFirst != 0 && incomingFirst < *first) {
		*first = incomingFirst
	}
	if incomingLast > *last {
		*last = incomingLast
	}
}

// unionStrings appends every element of b to a that isn't already present.
func unionStrings(a, b []string) []string {
	for _, x := range b {
		if !slices.Contains(a, x) {
			a = append(a, x)
		}
	}
	return a
}

// SetResolution records the reconciliation outcome for a fingerprint.
// Pass empty resolution to clear.
func (s *Store) SetResolution(fp string, res Resolution) {
	if s.Resolutions == nil {
		s.Resolutions = map[string]Resolution{}
	}
	if res == "" {
		delete(s.Resolutions, fp)
		return
	}
	s.Resolutions[fp] = res
}

// AddSuggestions appends new suggestion rows for fp, replacing any
// existing rows for that fingerprint. Top candidate first.
func (s *Store) AddSuggestions(fp string, suggestions []StateSuggestion) {
	kept := s.Suggestions[:0]
	for _, sg := range s.Suggestions {
		if sg.Fingerprint != fp {
			kept = append(kept, sg)
		}
	}
	s.Suggestions = append(kept, suggestions...)
}
