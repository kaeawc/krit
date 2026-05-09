package rename

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// ValidatePlan reports conflicts that would make the rename ambiguous or
// destructive: more than one declaration matching FromFQN, or an existing
// declaration already at ToFQN.
func ValidatePlan(plan Plan) error {
	if len(plan.Declarations) > 1 {
		return fmt.Errorf("rename: %s resolves to %d declarations; refusing to proceed", plan.Target.FromFQN, len(plan.Declarations))
	}
	if plan.Target.FromFQN == plan.Target.ToFQN {
		return fmt.Errorf("rename: from and to are identical")
	}
	return nil
}

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

// Plan identifies the declarations and reference sites a rename will
// touch, plus the per-file context Apply needs to rewrite imports and
// move files.
type Plan struct {
	Target       Target
	Declarations []scanner.Symbol
	References   []scanner.Reference
	Files        []string

	contexts    map[string]fileContext
	filesByPath map[string]*scanner.File
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

// BuildPlan computes a rename plan from the index. Use BuildPlanWithFiles
// when extra Java files need to participate (CodeIndex.Files only carries
// Kotlin files).
func BuildPlan(idx *scanner.CodeIndex, target Target) Plan {
	return BuildPlanWithFiles(idx, target, nil)
}

// BuildPlanWithFiles is BuildPlan plus extraFiles. References are kept
// only when their file's package/import context resolves the simple name
// to target.FromFQN.
func BuildPlanWithFiles(idx *scanner.CodeIndex, target Target, extraFiles []*scanner.File) Plan {
	plan := Plan{
		Target:      target,
		contexts:    make(map[string]fileContext),
		filesByPath: make(map[string]*scanner.File),
	}
	if idx == nil {
		return plan
	}

	for _, file := range idx.Files {
		plan.indexFile(file)
	}
	for _, file := range extraFiles {
		plan.indexFile(file)
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
			if ref.Name != target.FromName || ref.InComment {
				continue
			}
			if !plan.referenceMatchesTarget(ref) {
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

// indexFile records the parsed file and its derived context. Idempotent
// across duplicate paths.
func (p *Plan) indexFile(file *scanner.File) {
	if file == nil {
		return
	}
	if _, seen := p.contexts[file.Path]; seen {
		return
	}
	p.contexts[file.Path] = buildFileContext(file)
	p.filesByPath[file.Path] = file
}

// referenceMatchesTarget reports whether a simple-name reference resolves
// to target.FromFQN in its file's context. References from files we
// didn't parse (XML, etc.) are accepted by name only — Apply will skip
// them because they have no rewrite-safe byte range.
func (p Plan) referenceMatchesTarget(ref scanner.Reference) bool {
	ctx, ok := p.contexts[ref.File]
	if !ok {
		return true
	}
	return ctx.matchesFQN(p.Target.FromName, p.Target.FromFQN)
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
