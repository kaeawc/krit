package output

import (
	"fmt"
	"strings"
)

// FirCoverage is how much of a scan the FIR pass could analyze: the files
// whose FIR verdict is authoritative, the files it gated (a compiler error or
// crash, so Go's findings stand), and the files outside every JVM
// compilation. It is reported as summary.fir in JSON and as
// runs[0].properties.fir in SARIF, so CI can chart and gate on it.
type FirCoverage struct {
	Authoritative int `json:"authoritative"`
	Gated         int `json:"gated"`
	Excluded      int `json:"excluded"`
	// TopCauses are the most common gating reasons, most frequent first.
	// Unresolved references are grouped by the package prefix of the import
	// that names them, so a missing library shows up as one cause.
	TopCauses []FirGatingCause `json:"topCauses,omitempty"`
}

// FirGatingCause is one gating reason and the number of files it gated.
type FirGatingCause struct {
	Cause string `json:"cause"`
	Count int    `json:"count"`
}

// Ratio is the share of the checked files (authoritative plus gated) whose
// verdict is authoritative; 1 when FIR checked no file.
func (c FirCoverage) Ratio() float64 {
	checked := c.Authoritative + c.Gated
	if checked == 0 {
		return 1
	}
	return float64(c.Authoritative) / float64(checked)
}

// maxCausesInLine bounds the causes the one-line summary names.
const maxCausesInLine = 3

// FirCoverageLine is the non-verbose stderr summary of a scan in which FIR
// gated files, or "" when it gated none.
func FirCoverageLine(c FirCoverage) string {
	if c.Gated == 0 {
		return ""
	}
	checked := c.Authoritative + c.Gated
	line := fmt.Sprintf("FIR analyzed %s of %s files (%.0f%%); %s gated by compile errors, so Go-only findings stand for them",
		groupThousands(c.Authoritative), groupThousands(checked), 100*c.Ratio(), groupThousands(c.Gated))
	if len(c.TopCauses) > 0 {
		var causes []string
		for i, cause := range c.TopCauses {
			if i == maxCausesInLine {
				break
			}
			causes = append(causes, fmt.Sprintf("%s x%s", cause.Cause, groupThousands(cause.Count)))
		}
		line += " (top causes: " + strings.Join(causes, ", ") + ")"
	}
	return line + ". Run with -v for the list."
}

func groupThousands(n int) string {
	s := fmt.Sprint(n)
	if n < 0 {
		return "-" + groupThousands(-n)
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
