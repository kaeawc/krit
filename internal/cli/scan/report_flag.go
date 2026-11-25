package scan

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"
)

// reportFormats are the formats --report FORMAT=PATH accepts.
var reportFormats = []string{"json", "plain", "sarif", "checkstyle"}

// reportTarget is one --report FORMAT=PATH report.
type reportTarget struct {
	Format string
	Path   string
}

// reportFlag is the --report value. A bare FORMAT keeps the historical
// meaning (an alias for -f that takes precedence); FORMAT=PATH, repeatable,
// adds a report file written from the same analysis run as every other one.
type reportFlag struct {
	format  *string
	targets *[]reportTarget
}

func (r *reportFlag) String() string {
	if r == nil || r.format == nil {
		return ""
	}
	return *r.format
}

func (r *reportFlag) Set(value string) error {
	format, path, ok := strings.Cut(value, "=")
	if !ok {
		*r.format = value
		return nil
	}
	format = strings.TrimSpace(format)
	if !isReportFormat(format) {
		return fmt.Errorf("unknown report format %q in %q (want one of %s)", format, value, strings.Join(reportFormats, ", "))
	}
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("report %q has no path (want FORMAT=PATH)", value)
	}
	for _, existing := range *r.targets {
		if sameReportPath(existing.Path, path) {
			return fmt.Errorf("report path %s is given twice", path)
		}
	}
	*r.targets = append(*r.targets, reportTarget{Format: format, Path: path})
	return nil
}

func isReportFormat(format string) bool {
	for _, known := range reportFormats {
		if format == known {
			return true
		}
	}
	return false
}

func sameReportPath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return absA == absB
}

// primaryOutputRequested reports whether the scan writes its usual output
// (stdout or -o) next to the --report FORMAT=PATH files: always without such
// reports, and with them only when -o, -f/--format or a bare --report FORMAT
// asks for it explicitly, so a run that only names report files prints just
// the summary.
func primaryOutputRequested(f *scanFlags, fs *flag.FlagSet) bool {
	if len(f.Reports) == 0 {
		return true
	}
	explicit := explicitFlagSet(fs)
	return explicit["o"] || explicit["f"] || explicit["format"] || *f.Report != ""
}
