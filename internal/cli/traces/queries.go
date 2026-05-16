package traces

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/kaeawc/krit/internal/traces"
	"github.com/kaeawc/krit/internal/traces/reconcile"
)

// runOrphans lists static symbols present in the chosen snapshot's
// symbol list but never observed in the trace store.
func runOrphans(args []string) int {
	fs := flag.NewFlagSet("traces orphans", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	commitFlag := fs.String("commit", "", "snapshot sha (default: latest captured)")
	jsonFlag := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	repoRoot, code := resolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}
	store, symbols, _, code := loadStoreAndSymbols(repoRoot, *commitFlag)
	if code != 0 {
		return code
	}
	idx := reconcile.BuildIndex(symbols)
	observed := map[string]bool{}
	for _, r := range reconcile.Reconcile(idx, store.States) {
		if r.Resolution == traces.ResolvedExact && r.Match.FQN != "" {
			observed[r.Match.FQN] = true
		}
	}
	var orphans []string
	for _, s := range symbols {
		if s.FQN == "" || observed[s.FQN] {
			continue
		}
		orphans = append(orphans, s.FQN)
	}
	sort.Strings(orphans)
	if *jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"orphans": orphans, "count": len(orphans)})
		return 0
	}
	for _, fqn := range orphans {
		fmt.Println(fqn)
	}
	fmt.Fprintf(os.Stderr, "%d orphan symbols (%d/%d observed)\n",
		len(orphans), len(observed), len(symbols))
	return 0
}

// runPhantoms lists runtime states whose top symbol can't be
// reconciled to any static symbol (Resolution == Unresolved).
func runPhantoms(args []string) int {
	fs := flag.NewFlagSet("traces phantoms", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	commitFlag := fs.String("commit", "", "snapshot sha (default: latest captured)")
	jsonFlag := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	repoRoot, code := resolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}
	store, symbols, _, code := loadStoreAndSymbols(repoRoot, *commitFlag)
	if code != 0 {
		return code
	}
	idx := reconcile.BuildIndex(symbols)
	stateByFP := map[string]traces.RuntimeState{}
	for _, st := range store.States {
		stateByFP[st.Fingerprint] = st
	}
	type phantom struct {
		Fingerprint string         `json:"fingerprint"`
		TopSymbol   string         `json:"top_symbol"`
		Role        traces.RoleTag `json:"role"`
		Count       int            `json:"count"`
	}
	var out []phantom
	for _, r := range reconcile.Reconcile(idx, store.States) {
		if r.Resolution != traces.Unresolved {
			continue
		}
		st := stateByFP[r.Fingerprint]
		out = append(out, phantom{
			Fingerprint: r.Fingerprint,
			TopSymbol:   st.TopSymbol,
			Role:        st.Role,
			Count:       st.Count,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if *jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"phantoms": out, "count": len(out)})
		return 0
	}
	for _, p := range out {
		fmt.Printf("%s\t%s\trole=%s\tcount=%d\n", p.Fingerprint, p.TopSymbol, p.Role, p.Count)
	}
	fmt.Fprintf(os.Stderr, "%d phantom states\n", len(out))
	return 0
}

// runDivergence diffs the runtime transitions observed under one
// commit's ingest sources against another's.
func runDivergence(args []string) int {
	fs := flag.NewFlagSet("traces divergence", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := fs.String("repo", "", "repository root (default: cwd)")
	fromFlag := fs.String("from", "", "baseline commit sha (required)")
	toFlag := fs.String("to", "", "comparison commit sha (required)")
	jsonFlag := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *fromFlag == "" || *toFlag == "" {
		fmt.Fprintln(os.Stderr, "error: --from and --to are required")
		return 1
	}
	repoRoot, code := resolveRepoRoot(*repoFlag)
	if code != 0 {
		return code
	}
	store, err := traces.Load(repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fromIDs := sourceIDsForCommit(store, *fromFlag)
	toIDs := sourceIDsForCommit(store, *toFlag)
	if len(fromIDs) == 0 || len(toIDs) == 0 {
		fmt.Fprintf(os.Stderr, "error: missing ingest sources for from=%s to=%s\n", *fromFlag, *toFlag)
		return 1
	}
	onlyFrom := transitionsOnlyIn(store, fromIDs, toIDs)
	onlyTo := transitionsOnlyIn(store, toIDs, fromIDs)
	if *jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"from":      *fromFlag,
			"to":        *toFlag,
			"only_from": onlyFrom,
			"only_to":   onlyTo,
		})
		return 0
	}
	fmt.Fprintf(os.Stdout, "transitions only in %s: %d\n", *fromFlag, len(onlyFrom))
	fmt.Fprintf(os.Stdout, "transitions only in %s: %d\n", *toFlag, len(onlyTo))
	return 0
}

func sourceIDsForCommit(s *traces.Store, sha string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, src := range s.Sources {
		if src.CommitSHA == sha {
			out[src.ID] = struct{}{}
		}
	}
	return out
}

// transitionsOnlyIn returns transitions observed by at least one
// source in `inclusive` and by no source in `exclusive` — the
// "observed in this commit, but not the other" set.
func transitionsOnlyIn(s *traces.Store, inclusive, exclusive map[string]struct{}) []traces.RuntimeTransition {
	out := make([]traces.RuntimeTransition, 0, len(s.Transitions))
	for _, t := range s.Transitions {
		hasIn, hasEx := false, false
		for _, src := range t.Sources {
			if _, ok := inclusive[src]; ok {
				hasIn = true
			}
			if _, ok := exclusive[src]; ok {
				hasEx = true
			}
		}
		if hasIn && !hasEx {
			out = append(out, t)
		}
	}
	return out
}
