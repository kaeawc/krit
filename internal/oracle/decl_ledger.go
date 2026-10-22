package oracle

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/bits"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/scanner"
)

// declLedgerFile is the ledger's file name next to the oracle cache, under
// the same versioned root as the entries it validates.
const declLedgerFile = "decl-ledger.json"

// declLedgerVersion is the ledger's own schema version. Bump it when the
// shape of a record, the digest function, or what counts as a visible
// declaration changes: a ledger of another version is discarded and rebuilt,
// and the memoized digests in it go with it.
const declLedgerVersion = 1

// The declared-name ledger makes a cached KAA entry's validity a property of
// the entry rather than of one run.
//
// A krit-types entry's dependency closure records only the declarations its
// file already resolved to. A declaration added later in a scope the file
// can see (a new overload in its package, an imported name) changes what the
// file resolves to without touching its closure. The ledger tracks, per
// source path and keyed by the file's content hash, the visible top-level
// declarations the file makes: its package and each (name, signature). It is
// refreshed on every run from every source file the scan collects, before the
// oracle filter list is applied, so a file the filter drops still contributes
// its names, and a file analyzed in an earlier invocation (a subpath scan, a
// per-module run, an LSP request) is still known to a later one.
//
// Each cache entry records NameDigest: a digest of the ledger items its file
// mentions and can see, taken from files other than itself at analysis time.
// A later run recomputes that digest against the current ledger and demotes
// the entry to a miss when it moved. A declaration only moves a digest when it
// is new, deleted, or its signature changed, so editing a function body does
// not re-analyze every file that merely uses the same name.
//
// Recomputing a digest means lexing the file, so the result is memoized in
// the file's ledger record under the ledger's version (a hash of all items):
// while no declaration anywhere changes, a warm run reads no source beyond
// what hashing already reads.
type declLedger struct {
	path  string
	files map[string]*ledgerFile
	dirty bool

	// computed counts digests derived from source rather than served from a
	// memo, so a run's reads are observable.
	computed atomic.Int64

	mu      sync.Mutex
	indexed bool
	version string
	byName  map[string][]ledgerRef
	ops     []ledgerRef
}

type ledgerFile struct {
	Hash  string       `json:"h"`
	Pkg   string       `json:"p,omitempty"`
	Decls []ledgerDecl `json:"d,omitempty"`
	// MemoVersion and MemoDigest cache this file's digest (the one entries
	// compare against) for Hash under the ledger version MemoVersion.
	MemoVersion string `json:"ml,omitempty"`
	MemoDigest  string `json:"md,omitempty"`
}

type ledgerDecl struct {
	Name string `json:"n"`
	Sig  string `json:"s"`
	Op   bool   `json:"o,omitempty"`
}

type ledgerRef struct {
	fileHash, pkg, name, sig string
	operator                 bool
}

type declLedgerDisk struct {
	V     int                    `json:"v"`
	Files map[string]*ledgerFile `json:"files"`
}

// newDeclLedger returns an empty ledger that persists to path ("" keeps it in
// memory).
func newDeclLedger(path string) *declLedger {
	return &declLedger{path: path, files: map[string]*ledgerFile{}}
}

// loadDeclLedger reads the ledger at path. A missing, unreadable, corrupt, or
// other-version ledger yields an empty one: it only costs re-deriving names.
func loadDeclLedger(path string) *declLedger {
	l := newDeclLedger(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return l
	}
	var disk declLedgerDisk
	if err := json.Unmarshal(data, &disk); err != nil || disk.V != declLedgerVersion || disk.Files == nil {
		return l
	}
	l.files = disk.Files
	return l
}

// save writes the ledger when it changed. A failed write is not an error for
// the run: the next one re-derives what was lost.
func (l *declLedger) save() {
	if l == nil || l.path == "" || !l.dirty {
		return
	}
	data, err := json.Marshal(declLedgerDisk{V: declLedgerVersion, Files: l.files})
	if err != nil {
		return
	}
	if err := fsutil.WriteFileAtomic(l.path, data, 0o644); err == nil {
		l.dirty = false
	}
}

func ledgerKey(path string) string { return filepath.Clean(AbsolutePath(path)) }

// refreshDeclLedger loads the ledger at path and refreshes it from the Kotlin
// and Java sources a scan of sourceDirs collected. A whole-compilation
// backend refreshes every file's facts together, so it gets no ledger (nil,
// which every method treats as "nothing to check").
func refreshDeclLedger(opts InvocationOptions, path string, sourceDirs, kotlin, java []string, tracker perf.Tracker) *declLedger {
	if opts.Backend.ReturnsWholeCompilation() {
		return nil
	}
	l := loadDeclLedger(path)
	l.refresh(sourceDirs, append(append([]string(nil), kotlin...), java...), tracker)
	return l
}

// ledgerSourceFile is one collected source file to refresh.
type ledgerSourceFile struct {
	path string
	key  string
	hash string
	rec  *ledgerFile // non-nil when the existing record is current
	next *ledgerFile // freshly derived record
	err  error
}

// refresh brings the ledger in line with files, the Kotlin (.kt, .kts) and
// Java sources the scan of sourceDirs collected. A file whose content hash
// differs from its record is re-parsed for its declarations. A record under a
// scanned root that was not collected has left the scan (deleted, newly
// ignored); a record outside every root whose file no longer exists is also
// dropped. Other records are kept: they describe files an earlier, wider or
// different scan saw, which a later one may still need.
func (l *declLedger) refresh(sourceDirs, files []string, tracker perf.Tracker) {
	start := time.Now()
	work := make([]*ledgerSourceFile, len(files))
	for i, p := range files {
		work[i] = &ledgerSourceFile{path: p, key: ledgerKey(p)}
		work[i].rec = l.files[work[i].key]
	}
	runParallel(len(work), func(i int) {
		w := work[i]
		w.hash, w.err = ContentHash(w.path)
		if w.err != nil {
			return
		}
		if w.rec != nil && w.rec.Hash == w.hash {
			return
		}
		w.rec = nil
		w.next = deriveLedgerFile(w.path, w.hash)
	})

	collected := make(map[string]bool, len(work))
	parsed := 0
	for _, w := range work {
		if w.err != nil {
			continue
		}
		collected[w.key] = true
		if w.next != nil {
			l.files[w.key] = w.next
			l.dirty = true
			parsed++
		}
	}
	roots := make([]string, 0, len(sourceDirs))
	for _, d := range sourceDirs {
		roots = append(roots, ledgerKey(d))
	}
	dropped := 0
	for key := range l.files {
		if collected[key] {
			continue
		}
		if underAnyRoot(key, roots) || !fileExists(key) {
			delete(l.files, key)
			l.dirty = true
			dropped++
		}
	}
	l.mu.Lock()
	l.indexed = false
	l.mu.Unlock()
	addOracleEntry(tracker, "declLedgerRefresh", start, map[string]int64{
		"files":   int64(len(files)),
		"parsed":  int64(parsed),
		"dropped": int64(dropped),
	}, nil)
}

func underAnyRoot(path string, roots []string) bool {
	for _, r := range roots {
		if pathUnderRoot(path, r) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !os.IsNotExist(err)
}

// deriveLedgerFile parses path for its top-level declarations. A file that
// cannot be parsed, and a Kotlin script, declares nothing other files see.
func deriveLedgerFile(path, hash string) *ledgerFile {
	rec := &ledgerFile{Hash: hash}
	var pkg string
	var decls []scanner.TopLevelDecl
	switch {
	case strings.HasSuffix(path, ".kt"):
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			return rec
		}
		pkg, decls = scanner.TopLevelDeclaredNames(file)
	case strings.HasSuffix(path, ".java"):
		file, err := scanner.ParseJavaFile(context.Background(), path)
		if err != nil {
			return rec
		}
		pkg, decls = scanner.TopLevelJavaTypeNames(file)
	default:
		return rec
	}
	rec.Pkg = pkg
	for _, d := range decls {
		rec.Decls = append(rec.Decls, ledgerDecl{Name: d.Name, Sig: d.Sig, Op: d.Operator})
	}
	return rec
}

// runParallel runs fn(0..n-1) on a bounded set of workers.
func runParallel(n int, fn func(i int)) {
	workers := runtime.GOMAXPROCS(0)
	if workers > 8 {
		workers = 8
	}
	if workers > n {
		workers = n
	}
	if workers <= 1 {
		for i := 0; i < n; i++ {
			fn(i)
		}
		return
	}
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				fn(i)
			}
		}()
	}
	for i := 0; i < n; i++ {
		next <- i
	}
	close(next)
	wg.Wait()
}

// buildIndex derives the lookup tables and the ledger version from the
// records. The version is an order-independent hash of every item, so any
// added, removed, or re-signed declaration changes it.
func (l *declLedger) buildIndex() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.indexed {
		return
	}
	l.byName = map[string][]ledgerRef{}
	l.ops = nil
	var sum, mix uint64
	var buf [8]byte
	for _, rec := range l.files {
		for _, d := range rec.Decls {
			ref := ledgerRef{fileHash: rec.Hash, pkg: rec.Pkg, name: d.Name, sig: d.Sig, operator: d.Op}
			l.byName[d.Name] = append(l.byName[d.Name], ref)
			if d.Op {
				l.ops = append(l.ops, ref)
			}
			h := fnv.New64a()
			for _, part := range []string{rec.Hash, rec.Pkg, d.Name, d.Sig} {
				_, _ = h.Write([]byte(part))
				_, _ = h.Write([]byte{0})
			}
			v := h.Sum64()
			sum += v
			mix ^= bits.RotateLeft64(v, 17)
		}
	}
	binary.BigEndian.PutUint64(buf[:], sum)
	l.version = fmt.Sprintf("%x", buf)
	binary.BigEndian.PutUint64(buf[:], mix)
	l.version += fmt.Sprintf("%x", buf)
	l.indexed = true
}

// digest returns the digest of the ledger items content mentions and can see,
// ignoring items declared by files whose hash is selfHash, or "" when there
// are none. It is the value an entry stores (NameDigest) and later runs
// recompute.
func (l *declLedger) digest(content []byte, selfHash string) string {
	l.buildIndex()
	idents := allIdentifiers(content)
	var header scanner.SourceHeader
	headerRead := false
	seen := map[string]bool{}
	var items []string
	consider := func(ref ledgerRef) {
		if ref.fileHash == selfHash {
			return
		}
		item := ref.pkg + "\x00" + ref.name + "\x00" + ref.sig
		if seen[item] {
			return
		}
		if !headerRead {
			header = scanner.ParseSourceHeader(content)
			headerRead = true
		}
		if !declaredNameVisible(header, declEntry{pkg: ref.pkg, name: ref.name}, content) {
			return
		}
		seen[item] = true
		items = append(items, item)
	}
	for name := range idents {
		for _, ref := range l.byName[name] {
			consider(ref)
		}
	}
	for _, ref := range l.ops {
		if !idents[ref.name] && operatorGateOpen(ref.name, content, idents) {
			consider(ref)
		}
	}
	if len(items) == 0 {
		return ""
	}
	sort.Strings(items)
	return hashutil.HashHex([]byte(strings.Join(items, "\n")))[:16]
}

// operatorGateOpen reports whether content can reach the operator function
// name through syntax alone, without spelling it (see operatorGates).
func operatorGateOpen(name string, content []byte, idents map[string]bool) bool {
	gate := gatesFor(name)
	for _, raw := range gate.raw {
		if bytes.Contains(content, []byte(raw)) {
			return true
		}
	}
	return anyToken(idents, gate.tokens)
}

// currentDigest returns the digest of the file at path under the current
// ledger. The result is memoized in the file's record while its content hash
// still matches. ok is false when the file cannot be read.
func (l *declLedger) currentDigest(path string) (digest string, ok bool) {
	l.buildIndex()
	key := ledgerKey(path)
	l.mu.Lock()
	rec := l.files[key]
	version := l.version
	if rec != nil && rec.MemoVersion == version {
		d := rec.MemoDigest
		l.mu.Unlock()
		return d, true
	}
	l.mu.Unlock()
	hash, err := ContentHash(path)
	if err != nil {
		return "", false
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	digest = l.digest(content, hash)
	l.computed.Add(1)
	l.mu.Lock()
	if rec = l.files[key]; rec != nil && rec.Hash == hash {
		rec.MemoVersion, rec.MemoDigest = version, digest
		l.dirty = true
	}
	l.mu.Unlock()
	return digest, true
}

// stampNameDigests records, for every file the analysis answered for, the
// digest to store in its new cache entry.
func (l *declLedger) stampNameDigests(fresh *Data, deps *CacheDepsFile) {
	if l == nil || fresh == nil || deps == nil {
		return
	}
	paths := make([]string, 0, len(fresh.Files))
	for p := range fresh.Files {
		paths = append(paths, p)
	}
	digests := make([]string, len(paths))
	runParallel(len(paths), func(i int) {
		digests[i], _ = l.currentDigest(paths[i])
	})
	deps.NameDigests = make(map[string]string, len(paths))
	for i, p := range paths {
		if digests[i] != "" {
			deps.NameDigests[p] = digests[i]
		}
	}
}
