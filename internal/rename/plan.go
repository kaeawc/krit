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

// BuildPlan projects the current reference index into the declaration and
// reference candidates for a requested rename.
func BuildPlan(idx *scanner.CodeIndex, target Target) Plan {
	plan := Plan{Target: target}
	if idx == nil {
		return plan
	}

	files := make(map[string]bool)

	for _, sym := range idx.Symbols {
		if sym.Name != target.FromName {
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
