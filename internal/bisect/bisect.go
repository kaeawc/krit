// Package bisect explains failures by fusing location signals over the
// snapshot timeline. Given a breakage event and a [from, to] sha
// range, it walks the captured snapshots and returns ranked
// (commit, module, reason, confidence) candidates.
//
// The fusion model mirrors AutoMobile's fingerprint tiers: each signal
// is tagged with a tier-derived prior, the highest prior across tiers
// that agree on a (commit, module) wins, and that prior is boosted by
// the number of agreeing tiers. The output is a ranked list, not a
// single answer.
package bisect

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/breakage"
	"github.com/kaeawc/krit/internal/snapshot"
)

// Tier identifies the source of a location signal. Lower tiers are
// higher-confidence — stack frames beat heuristics, heuristics beat
// raw structural diffs, raw diffs beat historical co-occurrence.
type Tier int

const (
	TierStackFrame      Tier = 1
	TierTestOwnsModule  Tier = 2
	TierDiffTouches     Tier = 3
	TierHistoricalCoOcc Tier = 4
	tierCount                = 4
)

// Prior returns the per-tier base confidence used as the AutoMobile-
// style prior. Boosts (agreement count, distance to event sha) layer
// on top.
func Prior(t Tier) float64 {
	switch t {
	case TierStackFrame:
		return 0.95
	case TierTestOwnsModule:
		return 0.85
	case TierDiffTouches:
		return 0.60
	case TierHistoricalCoOcc:
		return 0.50
	}
	return 0.0
}

// Signal is one (commit, module) location candidate carrying a tier and
// a one-line reason. The Reason field shows up in CLI output verbatim;
// keep it short and self-explanatory.
type Signal struct {
	CommitSHA string
	Module    string
	Tier      Tier
	Reason    string
}

// Candidate is a fused location: one (commit, module) bucket and the
// agreeing signals that backed it. Confidence is in [0, 1].
type Candidate struct {
	CommitSHA  string   `json:"commit_sha"`
	Module     string   `json:"module"`
	Confidence float64  `json:"confidence"`
	Reasons    []string `json:"reasons"`
	Tiers      []Tier   `json:"tiers"`
}

// Input is the entry point's parameter bag. SnapshotsRoot is the
// `.krit/snapshots` directory; FromSHA and ToSHA bound the search
// range and need not be themselves captured (we walk captured manifests
// inside that range).
type Input struct {
	SnapshotsRoot string
	FromSHA       string
	ToSHA         string
	Event         breakage.Event
	// HistoricalEvents is the full event ledger; when non-nil the tier-4
	// (historical co-occurrence) signals draw from it.
	HistoricalEvents []breakage.Event
	// MaxResults caps the returned candidates. Zero means "no cap".
	MaxResults int
}

// Result is the ranked output. Candidates are sorted by Confidence
// descending, then by (CommitSHA, Module) ascending.
type Result struct {
	Event      breakage.Event `json:"event"`
	From       string         `json:"from"`
	To         string         `json:"to"`
	Candidates []Candidate    `json:"candidates"`
}

// Run executes a structural bisection over the captured snapshot
// timeline. When no captured manifests fall in [FromSHA, ToSHA] the
// result has no candidates and no error.
func Run(in Input) (*Result, error) {
	if in.SnapshotsRoot == "" {
		return nil, fmt.Errorf("bisect: SnapshotsRoot required")
	}
	manifests, err := snapshot.LoadManifests(in.SnapshotsRoot)
	if err != nil {
		return nil, fmt.Errorf("bisect: load manifests: %w", err)
	}
	timeline := orderManifests(manifests)

	// The tier-1 and tier-2 signals both inspect the event's blob; load
	// it once and pass it through. Errors are non-fatal — those signals
	// simply contribute nothing.
	var eventBlob *snapshot.Blob
	if in.Event.CommitSHA != "" {
		if b, err := snapshot.Load(in.SnapshotsRoot, in.Event.CommitSHA); err == nil {
			eventBlob = b
		}
	}

	var signals []Signal
	signals = append(signals, stackFrameSignals(in, eventBlob)...)
	signals = append(signals, testOwnsModuleSignals(in, eventBlob)...)
	signals = append(signals, diffTouchesSignals(in, timeline)...)
	signals = append(signals, historicalCoOccurrenceSignals(in)...)

	candidates := fuse(signals)
	if in.MaxResults > 0 && len(candidates) > in.MaxResults {
		candidates = candidates[:in.MaxResults]
	}
	return &Result{
		Event:      in.Event,
		From:       in.FromSHA,
		To:         in.ToSHA,
		Candidates: candidates,
	}, nil
}

// orderManifests sorts manifests by capture time ascending. Captures
// for un-resolvable commit times tie-break by sha, matching git log's
// behaviour for synthetic shas.
func orderManifests(manifests []snapshot.Manifest) []snapshot.Manifest {
	out := append([]snapshot.Manifest(nil), manifests...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].CapturedAt != out[j].CapturedAt {
			return out[i].CapturedAt < out[j].CapturedAt
		}
		return out[i].CommitSHA < out[j].CommitSHA
	})
	return out
}

// manifestsInRange returns the slice of manifests whose CommitSHA
// equals FromSHA, ToSHA, or lies strictly between them in capture-
// time order. When either bound is empty the open end is treated as
// unbounded.
func manifestsInRange(manifests []snapshot.Manifest, from, to string) []snapshot.Manifest {
	if from == "" && to == "" {
		return manifests
	}
	indexOf := func(sha string) int {
		if sha == "" {
			return -1
		}
		for i, m := range manifests {
			if m.CommitSHA == sha {
				return i
			}
		}
		return -1
	}
	fromIdx := indexOf(from)
	toIdx := indexOf(to)
	if fromIdx < 0 {
		fromIdx = 0
	}
	if toIdx < 0 {
		toIdx = len(manifests) - 1
	}
	if fromIdx > toIdx {
		fromIdx, toIdx = toIdx, fromIdx
	}
	return manifests[fromIdx : toIdx+1]
}

// stackFrameSignals attributes the event to its own commit when any
// frame maps to a captured symbol — the frame names a definition that
// exists at that sha.
func stackFrameSignals(in Input, blob *snapshot.Blob) []Signal {
	if blob == nil || len(in.Event.Frames) == 0 {
		return nil
	}
	idx := indexSymbolsByFQN(blob)
	for _, frame := range in.Event.Frames {
		frame = strings.TrimSpace(frame)
		if frame == "" {
			continue
		}
		if sym := idx.match(frame); sym != nil {
			mod := moduleForFile(blob, sym.File)
			return []Signal{{
				CommitSHA: in.Event.CommitSHA,
				Module:    mod,
				Tier:      TierStackFrame,
				Reason:    "stack frame " + frame + " maps to " + sym.FQN,
			}}
		}
	}
	return nil
}

// symbolIndex lets stack-frame matching run in O(frames) instead of
// O(frames * symbols) per Run. Frames carry forms like
// "com.acme.Order.place(Order.kt:42)"; match accepts the cleaned form
// being equal to an FQN, sharing its last segment, or being a method
// on an enclosing class FQN.
type symbolIndex struct {
	blob     *snapshot.Blob
	byFQN    map[string]*snapshot.Symbol
	bySuffix map[string]*snapshot.Symbol
}

func indexSymbolsByFQN(blob *snapshot.Blob) *symbolIndex {
	idx := &symbolIndex{
		blob:     blob,
		byFQN:    make(map[string]*snapshot.Symbol, len(blob.Symbols)),
		bySuffix: make(map[string]*snapshot.Symbol, len(blob.Symbols)),
	}
	for i := range blob.Symbols {
		s := &blob.Symbols[i]
		if s.FQN == "" {
			continue
		}
		idx.byFQN[s.FQN] = s
		last := s.FQN
		if dot := strings.LastIndexByte(last, '.'); dot >= 0 {
			last = last[dot+1:]
		}
		idx.bySuffix[last] = s
	}
	return idx
}

func (idx *symbolIndex) match(frame string) *snapshot.Symbol {
	clean := frame
	if i := strings.IndexByte(clean, '('); i >= 0 {
		clean = clean[:i]
	}
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return nil
	}
	if s, ok := idx.byFQN[clean]; ok {
		return s
	}
	if dot := strings.LastIndexByte(clean, '.'); dot < 0 {
		if s, ok := idx.bySuffix[clean]; ok {
			return s
		}
	}
	// Prefix match: walk the dot-separated prefixes of clean, longest
	// first, so a class FQN wins over its enclosing package.
	for clean != "" {
		dot := strings.LastIndexByte(clean, '.')
		if dot < 0 {
			break
		}
		clean = clean[:dot]
		if s, ok := idx.byFQN[clean]; ok {
			return s
		}
	}
	return nil
}

func moduleForFile(blob *snapshot.Blob, path string) string {
	for i := range blob.Files {
		if blob.Files[i].Path == path {
			return blob.Files[i].Module
		}
	}
	return ""
}

// testOwnsModuleSignals applies the heuristic that a test under
// <mod>/src/test|androidTest|integrationTest/... owns the matching
// <mod>/src/main module.
func testOwnsModuleSignals(in Input, blob *snapshot.Blob) []Signal {
	if blob == nil || in.Event.File == "" {
		return nil
	}
	mod := moduleForFile(blob, in.Event.File)
	if mod == "" {
		mod = inferModuleFromTestPath(blob, in.Event.File)
	}
	if mod == "" {
		return nil
	}
	return []Signal{{
		CommitSHA: in.Event.CommitSHA,
		Module:    mod,
		Tier:      TierTestOwnsModule,
		Reason:    "test file " + in.Event.File + " owns module " + mod,
	}}
}

func inferModuleFromTestPath(blob *snapshot.Blob, path string) string {
	for _, marker := range []string{"/src/test/", "/src/androidTest/", "/src/integrationTest/"} {
		i := strings.Index(path, marker)
		if i <= 0 {
			continue
		}
		prefix := path[:i]
		for _, m := range blob.Modules {
			// The test file's prefix (the absolute path up to the
			// /src/test/ marker) must equal the module dir or have it
			// as a path suffix. The previous middle clause
			// `HasSuffix(m.Dir, prefix)` was backwards and would
			// attribute the test to any sibling module whose dir
			// happened to end with the test-prefix string.
			if m.Dir == prefix || strings.HasSuffix(prefix, strings.TrimPrefix(m.Dir, "./")) {
				return m.Path
			}
		}
	}
	return ""
}

// diffTouchesSignals attributes each module touched between adjacent
// captured snapshots to the "to" side — the first sha where the
// change is visible.
func diffTouchesSignals(in Input, timeline []snapshot.Manifest) []Signal {
	bounded := manifestsInRange(timeline, in.FromSHA, in.ToSHA)
	if len(bounded) < 2 {
		return nil
	}
	var out []Signal
	for i := 1; i < len(bounded); i++ {
		from := bounded[i-1]
		to := bounded[i]
		d, err := snapshot.Diff(in.SnapshotsRoot, from.CommitSHA, to.CommitSHA)
		if err != nil || d == nil {
			continue
		}
		touched := touchedModules(d)
		for mod := range touched {
			out = append(out, Signal{
				CommitSHA: to.CommitSHA,
				Module:    mod,
				Tier:      TierDiffTouches,
				Reason:    "structural delta touches " + mod + " at " + shortSHA(to.CommitSHA),
			})
		}
	}
	return out
}

func touchedModules(d *snapshot.DiffResult) map[string]bool {
	out := make(map[string]bool)
	for _, f := range d.AddedFiles {
		if f.Module != "" {
			out[f.Module] = true
		}
	}
	for _, f := range d.RemovedFiles {
		if f.Module != "" {
			out[f.Module] = true
		}
	}
	for _, e := range d.AddedEdges {
		if e.From != "" {
			out[e.From] = true
		}
	}
	for _, e := range d.RemovedEdges {
		if e.From != "" {
			out[e.From] = true
		}
	}
	for mod := range d.ModuleMetrics {
		out[mod] = true
	}
	return out
}

// historicalCoOccurrenceSignals suggests modules that previous events
// of the same kind or signature already implicated. Tier-4 only
// proposes the module, not the triggering sha, so the candidate
// commit is the event's own.
func historicalCoOccurrenceSignals(in Input) []Signal {
	if len(in.HistoricalEvents) == 0 || in.Event.CommitSHA == "" {
		return nil
	}
	var out []Signal
	for _, h := range in.HistoricalEvents {
		if h.ID == in.Event.ID {
			continue
		}
		if h.Module == "" {
			continue
		}
		if !sameKindOrSignature(h, in.Event) {
			continue
		}
		out = append(out, Signal{
			CommitSHA: in.Event.CommitSHA,
			Module:    h.Module,
			Tier:      TierHistoricalCoOcc,
			Reason:    "module " + h.Module + " previously implicated by " + h.FailureKind,
		})
	}
	return out
}

func sameKindOrSignature(a, b breakage.Event) bool {
	if a.FailureKind == b.FailureKind {
		return true
	}
	if a.Signature != "" && a.Signature == b.Signature {
		return true
	}
	return false
}

// fuse buckets signals by (commit, module), keeps the max prior per
// bucket, and boosts it toward 1 by the number of agreeing tiers
// beyond the first. One tier alone never reaches certainty.
func fuse(signals []Signal) []Candidate {
	type key struct{ commit, module string }
	type bucket struct {
		maxPrior float64
		tiers    map[Tier]struct{}
		reasons  []string
	}
	buckets := make(map[key]*bucket)
	for _, s := range signals {
		if s.Module == "" {
			continue
		}
		k := key{commit: s.CommitSHA, module: s.Module}
		b := buckets[k]
		if b == nil {
			b = &bucket{tiers: make(map[Tier]struct{})}
			buckets[k] = b
		}
		if p := Prior(s.Tier); p > b.maxPrior {
			b.maxPrior = p
		}
		if _, dup := b.tiers[s.Tier]; !dup {
			b.tiers[s.Tier] = struct{}{}
			b.reasons = append(b.reasons, s.Reason)
		}
	}

	out := make([]Candidate, 0, len(buckets))
	for k, b := range buckets {
		agreements := len(b.tiers)
		boost := 0.0
		if agreements > 1 {
			boost = (1.0 - b.maxPrior) * float64(agreements-1) / float64(tierCount-1)
		}
		conf := b.maxPrior + boost
		if conf > 1.0 {
			conf = 1.0
		}
		tiers := tierKeys(b.tiers)
		out = append(out, Candidate{
			CommitSHA:  k.commit,
			Module:     k.module,
			Confidence: conf,
			Reasons:    append([]string(nil), b.reasons...),
			Tiers:      tiers,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		if out[i].CommitSHA != out[j].CommitSHA {
			return out[i].CommitSHA < out[j].CommitSHA
		}
		return out[i].Module < out[j].Module
	})
	return out
}

func tierKeys(m map[Tier]struct{}) []Tier {
	out := make([]Tier, 0, len(m))
	for t := range m {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
