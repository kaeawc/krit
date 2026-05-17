// Package suggestreviewers implements the `krit suggest-reviewers`
// subcommand: parse CODEOWNERS, follow the cross-file reference index
// from the symbols changed in a PR, and propose reviewer teams that own
// callers of those symbols (in addition to teams that own the changed
// files directly).
package suggestreviewers

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/arch"
	"github.com/kaeawc/krit/internal/cli/clishared"
	"github.com/kaeawc/krit/internal/proc"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	bestKindCallers = "callers"
	bestKindDirect  = "direct"
)

func Run(args []string) int {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return runWith(proc.Default, root, os.Stdout, os.Stderr, args)
}

func runWith(runner proc.Runner, root string, stdout, stderr *os.File, args []string) int {
	fs := flag.NewFlagSet("suggest-reviewers", flag.ContinueOnError)
	fs.SetOutput(stderr)
	base := fs.String("base", "main", "Git base ref to diff against")
	jsonFlag := fs.Bool("json", false, "Emit JSON instead of plain text")
	topN := fs.Int("top", 5, "Maximum number of suggested teams")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	coPath := FindCodeownersFile(root)
	if coPath == "" {
		fmt.Fprintln(stderr, "error: CODEOWNERS not found at CODEOWNERS, .github/CODEOWNERS, or docs/CODEOWNERS")
		return 1
	}
	co, err := ParseCodeownersFile(coPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	changedRel, err := changedFiles(runner, root, *base)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	suggestion := buildSuggestions(root, co, changedRel)
	suggestion.TopN = *topN
	if *jsonFlag {
		return emitJSON(stdout, suggestion)
	}
	return emitText(stdout, suggestion)
}

type suggestion struct {
	ChangedFiles []string         `json:"changed_files"`
	CallerTotal  map[string]int   `json:"caller_total,omitempty"` // fqn -> total consumer files (across all teams)
	Teams        []teamSuggestion `json:"teams"`
	TopN         int              `json:"-"`
}

type teamSuggestion struct {
	Team        string             `json:"team"`
	CallerHits  map[string]int     `json:"caller_hits,omitempty"` // fqn -> consumer files owned by team
	DirectFiles []string           `json:"direct_files,omitempty"`
	Score       int                `json:"score"`
	Best        bestRecommendation `json:"best"`
}

type bestRecommendation struct {
	Kind   string `json:"kind"` // bestKindCallers or bestKindDirect
	Symbol string `json:"symbol,omitempty"`
	File   string `json:"file,omitempty"`
	Hits   int    `json:"hits,omitempty"`
	Total  int    `json:"total,omitempty"`
}

func buildSuggestions(root string, co *Codeowners, changedRel []string) suggestion {
	sort.Strings(changedRel)

	// Direct file ownership for the changed files themselves.
	directByTeam := map[string]map[string]bool{}
	for _, rel := range changedRel {
		for _, owner := range co.Owners(rel) {
			if directByTeam[owner] == nil {
				directByTeam[owner] = map[string]bool{}
			}
			directByTeam[owner][rel] = true
		}
	}

	// Caller-based suggestions need an index over project Kotlin sources.
	changedFqns := changedFqnsFromFiles(root, changedRel)
	callerHits := map[string]map[string]int{} // team -> fqn -> consumer files owned
	callerTotal := map[string]int{}           // fqn -> total consumer files
	if len(changedFqns) > 0 {
		idx := buildIndex(root)
		if idx != nil {
			declFiles := declarationFilesByName(idx)
			for _, fqn := range changedFqns {
				name := clishared.SimpleName(fqn)
				if name == "" {
					continue
				}
				total := 0
				for f := range idx.ReferenceFiles(name) {
					if declFiles[name][f] {
						continue
					}
					total++
					rel, err := filepath.Rel(root, f)
					if err != nil {
						continue
					}
					rel = filepath.ToSlash(rel)
					for _, owner := range co.Owners(rel) {
						if callerHits[owner] == nil {
							callerHits[owner] = map[string]int{}
						}
						callerHits[owner][fqn]++
					}
				}
				callerTotal[fqn] = total
			}
		}
	}

	// Merge owners and rank.
	owners := map[string]bool{}
	for o := range directByTeam {
		owners[o] = true
	}
	for o := range callerHits {
		owners[o] = true
	}

	teams := make([]teamSuggestion, 0, len(owners))
	for owner := range owners {
		t := teamSuggestion{Team: owner}
		if hits := callerHits[owner]; len(hits) > 0 {
			t.CallerHits = hits
			for _, n := range hits {
				t.Score += n
			}
		}
		if files := directByTeam[owner]; len(files) > 0 {
			t.DirectFiles = sortedKeys(files)
			t.Score += len(files)
		}
		t.Best = bestForTeam(t, callerTotal)
		teams = append(teams, t)
	}

	sort.Slice(teams, func(i, j int) bool {
		if teams[i].Score != teams[j].Score {
			return teams[i].Score > teams[j].Score
		}
		return teams[i].Team < teams[j].Team
	})

	return suggestion{ChangedFiles: changedRel, CallerTotal: callerTotal, Teams: teams}
}

func bestForTeam(t teamSuggestion, callerTotal map[string]int) bestRecommendation {
	if len(t.CallerHits) > 0 {
		var bestFqn string
		bestHits := -1
		bestTotal := 0
		for fqn, hits := range t.CallerHits {
			if hits > bestHits || (hits == bestHits && fqn < bestFqn) {
				bestFqn = fqn
				bestHits = hits
				bestTotal = callerTotal[fqn]
			}
		}
		return bestRecommendation{Kind: bestKindCallers, Symbol: bestFqn, Hits: bestHits, Total: bestTotal}
	}
	if len(t.DirectFiles) > 0 {
		return bestRecommendation{Kind: bestKindDirect, File: t.DirectFiles[0]}
	}
	return bestRecommendation{}
}

func emitText(out *os.File, s suggestion) int {
	if len(s.Teams) == 0 {
		fmt.Fprintln(out, "No reviewers suggested (no matching CODEOWNERS rules).")
		return 0
	}
	fmt.Fprintln(out, "Suggested reviewers:")
	limit := s.TopN
	if limit <= 0 || limit > len(s.Teams) {
		limit = len(s.Teams)
	}
	for _, t := range s.Teams[:limit] {
		switch t.Best.Kind {
		case bestKindCallers:
			fmt.Fprintf(out, "  %s\t(owns %d/%d callers of %s)\n",
				t.Team, t.Best.Hits, t.Best.Total, displaySymbol(t.Best.Symbol))
		case bestKindDirect:
			fmt.Fprintf(out, "  %s\t(owns %s — direct file owner)\n", t.Team, t.Best.File)
		default:
			fmt.Fprintf(out, "  %s\n", t.Team)
		}
	}
	return 0
}

func emitJSON(out *os.File, s suggestion) int {
	limit := s.TopN
	if limit <= 0 || limit > len(s.Teams) {
		limit = len(s.Teams)
	}
	view := s
	view.Teams = append([]teamSuggestion(nil), s.Teams[:limit]...)
	if view.ChangedFiles == nil {
		view.ChangedFiles = []string{}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(view); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

// changedFiles lists repo-relative forward-slash paths reported by
// `git diff --name-only --diff-filter=ACMR <base>`.
func changedFiles(runner proc.Runner, root, base string) ([]string, error) {
	res, err := runner.Run(context.Background(), proc.Cmd{
		Name: "git",
		Args: []string{"diff", "--name-only", "--diff-filter=ACMR", base},
		Dir:  root,
	})
	if err != nil {
		return nil, fmt.Errorf("git diff %s: %w", base, err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("git diff %s: exit %d: %s", base, res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(res.Stdout)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, filepath.ToSlash(line))
	}
	return out, nil
}

// changedFqnsFromFiles returns the union of FQNs declared in each
// changed Kotlin file that still exists on disk. Java files are ignored:
// this command's caller analysis is Kotlin-rooted because the cross-file
// index resolves Kotlin declarations.
func changedFqnsFromFiles(root string, changedRel []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, rel := range changedRel {
		if !strings.HasSuffix(rel, ".kt") {
			continue
		}
		full := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(full)
		if err != nil || info.IsDir() {
			continue
		}
		f, err := scanner.ParseFile(context.Background(), full)
		if err != nil {
			continue
		}
		for _, sig := range arch.ExtractAbiSignatures([]*scanner.File{f}) {
			if seen[sig.FQN] {
				continue
			}
			seen[sig.FQN] = true
			out = append(out, sig.FQN)
		}
	}
	return out
}

func buildIndex(root string) *scanner.CodeIndex {
	paths, err := scanner.CollectKotlinFiles([]string{root}, nil)
	if err != nil {
		return nil
	}
	files, _ := scanner.ScanFiles(context.Background(), paths, runtime.NumCPU())
	return scanner.BuildIndex(files, runtime.NumCPU())
}

// declarationFilesByName indexes Symbols by simple name so callers can
// exclude the declaring file from "consumer" counts when a symbol's own
// declaration site otherwise looks like a reference.
func declarationFilesByName(idx *scanner.CodeIndex) map[string]map[string]bool {
	m := map[string]map[string]bool{}
	for _, sym := range idx.Symbols {
		if m[sym.Name] == nil {
			m[sym.Name] = map[string]bool{}
		}
		m[sym.Name][sym.File] = true
	}
	return m
}

// displaySymbol shortens long FQNs for the text format by keeping the
// last two segments (Owner.method); JSON output keeps the full FQN.
func displaySymbol(fqn string) string {
	parts := strings.Split(fqn, ".")
	if len(parts) <= 2 {
		return fqn
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
