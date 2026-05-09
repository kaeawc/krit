package rename

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

var ErrApplyNotImplemented = errors.New("rename apply is not implemented yet")

// Target describes a requested FQN rename.
type Target struct {
	FromFQN  string
	ToFQN    string
	FromName string
	ToName   string
}

// FromPackage returns the package portion of FromFQN (everything before the
// final dot), or the empty string for an unqualified target.
func (t Target) FromPackage() string {
	idx := strings.LastIndex(t.FromFQN, ".")
	if idx <= 0 {
		return ""
	}
	return t.FromFQN[:idx]
}

// ToPackage returns the package portion of ToFQN.
func (t Target) ToPackage() string {
	idx := strings.LastIndex(t.ToFQN, ".")
	if idx <= 0 {
		return ""
	}
	return t.ToFQN[:idx]
}

// PackageChanged reports whether the rename moves the symbol to a new
// package — i.e. its FQN parent changed.
func (t Target) PackageChanged() bool {
	return t.FromPackage() != t.ToPackage()
}

// Summary is a compact view of the current rename plan.
type Summary struct {
	Declarations int
	References   int
	Files        int
}

// Plan is the minimum viable rename substrate: it identifies the declarations
// and reference sites that a future apply phase will need to rewrite.
type Plan struct {
	Target       Target
	Declarations []scanner.Symbol
	References   []scanner.Reference
	Files        []string
	// Contexts is the per-file package/import context for every file that
	// appears in Files (when known). Apply uses it to rewrite import
	// statements and detect package-only renames.
	Contexts map[string]FileContext

	// cachedFiles holds the parsed *scanner.File objects from the index
	// so Apply can reuse already-loaded content instead of re-reading
	// from disk. Internal — not part of the persisted plan shape.
	cachedFiles []*scanner.File
}

// ParseTarget validates the FQN arguments and extracts the simple names used by
// the current reference index.
func ParseTarget(fromFQN, toFQN string) (Target, error) {
	fromFQN = strings.TrimSpace(fromFQN)
	toFQN = strings.TrimSpace(toFQN)
	if fromFQN == "" || toFQN == "" {
		return Target{}, fmt.Errorf("rename requires both <from-fqn> and <to-fqn>")
	}
	if fromFQN == toFQN {
		return Target{}, fmt.Errorf("rename source and destination must differ")
	}

	fromName, ok := simpleName(fromFQN)
	if !ok {
		return Target{}, fmt.Errorf("rename source must be a fully qualified name: %s", fromFQN)
	}
	toName, ok := simpleName(toFQN)
	if !ok {
		return Target{}, fmt.Errorf("rename destination must be a fully qualified name: %s", toFQN)
	}

	return Target{
		FromFQN:  fromFQN,
		ToFQN:    toFQN,
		FromName: fromName,
		ToName:   toName,
	}, nil
}

// BuildPlan is a convenience wrapper around BuildPlanWithFiles with no
// extra files beyond what the index carries.
func BuildPlan(idx *scanner.CodeIndex, target Target) Plan {
	return BuildPlanWithFiles(idx, target, nil)
}

// BuildPlanWithFiles projects the current reference index into the declaration
// and reference candidates for a requested rename. References are filtered to
// only those that resolve to target.FromFQN in their file's package and
// import context. extraFiles supplies parsed *scanner.File objects that the
// CodeIndex does not itself store (Java files, in particular) so Apply can
// rewrite their package declarations and imports.
func BuildPlanWithFiles(idx *scanner.CodeIndex, target Target, extraFiles []*scanner.File) Plan {
	plan := Plan{Target: target, Contexts: make(map[string]FileContext)}
	if idx == nil {
		return plan
	}

	addFile := func(file *scanner.File) {
		if file == nil {
			return
		}
		if _, seen := plan.Contexts[file.Path]; seen {
			return
		}
		plan.Contexts[file.Path] = BuildFileContext(file)
		plan.cachedFiles = append(plan.cachedFiles, file)
	}
	for _, file := range idx.Files {
		addFile(file)
	}
	for _, file := range extraFiles {
		addFile(file)
	}

	files := make(map[string]bool)

	for _, sym := range idx.Symbols {
		if sym.Name != target.FromName {
			continue
		}
		if sym.FQN != "" && sym.FQN != target.FromFQN {
			continue
		}
		plan.Declarations = append(plan.Declarations, sym)
		files[sym.File] = true
	}

	if idx.MayHaveReference(target.FromName) {
		for _, ref := range idx.References {
			if ref.Name != target.FromName {
				continue
			}
			if ref.InComment {
				continue
			}
			if !referenceMatchesTarget(ref, target, plan.Contexts) {
				continue
			}
			plan.References = append(plan.References, ref)
			files[ref.File] = true
		}
	}

	sort.Slice(plan.Declarations, func(i, j int) bool {
		if plan.Declarations[i].File != plan.Declarations[j].File {
			return plan.Declarations[i].File < plan.Declarations[j].File
		}
		if plan.Declarations[i].Line != plan.Declarations[j].Line {
			return plan.Declarations[i].Line < plan.Declarations[j].Line
		}
		return plan.Declarations[i].Name < plan.Declarations[j].Name
	})
	sort.Slice(plan.References, func(i, j int) bool {
		if plan.References[i].File != plan.References[j].File {
			return plan.References[i].File < plan.References[j].File
		}
		if plan.References[i].Line != plan.References[j].Line {
			return plan.References[i].Line < plan.References[j].Line
		}
		return plan.References[i].Name < plan.References[j].Name
	})
	for file := range files {
		plan.Files = append(plan.Files, file)
	}
	sort.Strings(plan.Files)

	return plan
}

// Summary returns deterministic aggregate counts for the plan.
func (p Plan) Summary() Summary {
	return Summary{
		Declarations: len(p.Declarations),
		References:   len(p.References),
		Files:        len(p.Files),
	}
}

// CandidateCount is the number of declaration and reference sites currently
// identified by the planner.
func (p Plan) CandidateCount() int {
	summary := p.Summary()
	return summary.Declarations + summary.References
}

// referenceMatchesTarget reports whether a simple-name reference should be
// counted as resolving to target.FromFQN. With no per-file context (e.g.
// XML, or a reference from a file the index didn't parse) the reference is
// accepted by name only — Apply will skip it because it has no
// rewrite-safe byte range.
func referenceMatchesTarget(ref scanner.Reference, target Target, contexts map[string]FileContext) bool {
	ctx, ok := contexts[ref.File]
	if !ok {
		return true
	}
	return ctx.MatchesFQN(target.FromName, target.FromFQN)
}

func simpleName(fqn string) (string, bool) {
	if strings.HasPrefix(fqn, ".") || strings.HasSuffix(fqn, ".") {
		return "", false
	}
	parts := strings.Split(fqn, ".")
	if len(parts) < 2 {
		return "", false
	}
	name := strings.TrimSpace(parts[len(parts)-1])
	if name == "" {
		return "", false
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return "", false
		}
	}
	return name, true
}
