package traces

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/kaeawc/krit/internal/scanner"
	snap "github.com/kaeawc/krit/internal/snapshot"
	"github.com/kaeawc/krit/internal/traces"
	"github.com/kaeawc/krit/internal/traces/reconcile"
)

// overlayDoc is the JSON shape emitted by `krit traces overlay --json`.
// It combines the static symbol set with the runtime states, the
// reconcile resolution, and the cross-state transitions.
type overlayDoc struct {
	Commit      string                     `json:"commit,omitempty"`
	StaticNodes []overlayStaticNode        `json:"static_nodes"`
	Runtime     []overlayRuntimeNode       `json:"runtime_states"`
	Transitions []traces.RuntimeTransition `json:"transitions"`
}

type overlayStaticNode struct {
	FQN      string `json:"fqn"`
	Kind     string `json:"kind,omitempty"`
	File     string `json:"file,omitempty"`
	Observed bool   `json:"observed"`
}

type overlayRuntimeNode struct {
	Fingerprint string                   `json:"fingerprint"`
	TopSymbol   string                   `json:"top_symbol"`
	Role        traces.RoleTag           `json:"role"`
	Count       int                      `json:"count"`
	Resolution  traces.Resolution        `json:"resolution"`
	Matched     string                   `json:"matched_fqn,omitempty"`
	Suggestions []traces.StateSuggestion `json:"suggestions,omitempty"`
}

func runOverlay(args []string) int {
	fs := flag.NewFlagSet("traces overlay", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	commitFlag := fs.String("commit", "", "snapshot sha to reconcile against (default: latest captured)")
	jsonFlag := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	repoRoot, code := resolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}
	store, symbols, sha, code := loadStoreAndSymbols(repoRoot, *commitFlag)
	if code != 0 {
		return code
	}
	idx := reconcile.BuildIndex(symbols)
	results := reconcile.Reconcile(idx, store.States)
	store.Suggestions = store.Suggestions[:0]
	store.Resolutions = map[string]traces.Resolution{}
	for _, r := range results {
		store.SetResolution(r.Fingerprint, r.Resolution)
		if len(r.Suggestions) > 0 {
			store.AddSuggestions(r.Fingerprint, r.Suggestions)
		}
	}
	if err := store.Save(repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if *jsonFlag {
		return emitOverlayJSON(store, results, symbols, sha)
	}
	return emitOverlayHuman(store, results)
}

func emitOverlayJSON(store *traces.Store, results []reconcile.Result, symbols []scanner.Symbol, sha string) int {
	observed := map[string]bool{}
	for _, r := range results {
		if r.Resolution == traces.ResolvedExact && r.Match.FQN != "" {
			observed[r.Match.FQN] = true
		}
	}
	doc := overlayDoc{
		Commit:      sha,
		StaticNodes: make([]overlayStaticNode, 0, len(symbols)),
	}
	for _, s := range symbols {
		if s.FQN == "" {
			continue
		}
		doc.StaticNodes = append(doc.StaticNodes, overlayStaticNode{
			FQN:      s.FQN,
			Kind:     s.Kind,
			File:     s.File,
			Observed: observed[s.FQN],
		})
	}
	sort.Slice(doc.StaticNodes, func(i, j int) bool { return doc.StaticNodes[i].FQN < doc.StaticNodes[j].FQN })

	stateByFP := map[string]traces.RuntimeState{}
	for _, st := range store.States {
		stateByFP[st.Fingerprint] = st
	}
	for _, r := range results {
		st := stateByFP[r.Fingerprint]
		node := overlayRuntimeNode{
			Fingerprint: r.Fingerprint,
			TopSymbol:   st.TopSymbol,
			Role:        st.Role,
			Count:       st.Count,
			Resolution:  r.Resolution,
		}
		if r.Resolution == traces.ResolvedExact {
			node.Matched = r.Match.FQN
		}
		if len(r.Suggestions) > 0 {
			node.Suggestions = r.Suggestions
		}
		doc.Runtime = append(doc.Runtime, node)
	}
	doc.Transitions = store.Transitions
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func emitOverlayHuman(store *traces.Store, results []reconcile.Result) int {
	exact, fuzzy, unresolved := 0, 0, 0
	for _, r := range results {
		switch r.Resolution {
		case traces.ResolvedExact:
			exact++
		case traces.ResolvedFuzzy:
			fuzzy++
		case traces.Unresolved:
			unresolved++
		}
	}
	fmt.Fprintf(os.Stdout, "runtime states: %d  (exact %d, fuzzy %d, unresolved %d)\n",
		len(results), exact, fuzzy, unresolved)
	fmt.Fprintf(os.Stdout, "transitions:    %d\n", len(store.Transitions))
	fmt.Fprintf(os.Stdout, "sources:        %d\n", len(store.Sources))
	return 0
}

// loadStoreAndSymbols opens the per-repo store and the symbol list
// from the requested (or latest-captured) snapshot. Returns the
// resolved sha so callers can stamp it on output.
func loadStoreAndSymbols(repoRoot, commitFlag string) (*traces.Store, []scanner.Symbol, string, int) {
	store, err := traces.Load(repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return nil, nil, "", 1
	}
	root := snap.SnapshotsDir(repoRoot)
	sha := commitFlag
	if sha == "" {
		manifests, err := snap.LoadManifests(root)
		if err != nil || len(manifests) == 0 {
			fmt.Fprintf(os.Stderr, "error: no snapshots captured; run `krit snapshot capture` first\n")
			return nil, nil, "", 1
		}
		// Most-recent capture wins.
		sort.Slice(manifests, func(i, j int) bool { return manifests[i].CapturedAt > manifests[j].CapturedAt })
		sha = manifests[0].CommitSHA
	}
	blob, err := snap.Load(root, sha)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load snapshot %s: %v\n", sha, err)
		return nil, nil, "", 1
	}
	symbols := convertSymbols(blob.Symbols)
	return store, symbols, sha, 0
}

// convertSymbols projects snapshot.Symbol onto scanner.Symbol so the
// reconcile.Index can index them. Only the fields reconcile reads are
// populated.
func convertSymbols(in []snap.Symbol) []scanner.Symbol {
	out := make([]scanner.Symbol, len(in))
	for i, s := range in {
		out[i] = scanner.Symbol{
			Name:      s.Name,
			Kind:      s.Kind,
			File:      s.File,
			Line:      s.Line,
			Package:   s.Package,
			FQN:       s.FQN,
			Owner:     s.Owner,
			Signature: s.Signature,
		}
	}
	return out
}
