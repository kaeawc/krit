package oracle

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/hashutil"
)

// CallTargetFilterSummary is the Go-side contract for narrowing JVM
// resolveToCall() work. Enabled=false means at least one active rule needs
// broad call targets, so krit-types must preserve the pre-filter behavior.
type CallTargetFilterSummary struct {
	Enabled     bool
	DisabledBy  []string
	CalleeNames []string
	TargetFQNs  []string
	Fingerprint string
}

type callTargetFilterJSON struct {
	Version     int      `json:"version"`
	Mode        string   `json:"mode"`
	CalleeNames []string `json:"calleeNames"`
	TargetFQNs  []string `json:"targetFqns,omitempty"`
}

// FinalizeCallTargetFilter sorts, deduplicates, derives simple callee names
// from FQNs, and computes a stable fingerprint. Callers that build summaries
// directly should run this before writing or passing the filter onward.
func FinalizeCallTargetFilter(summary CallTargetFilterSummary) CallTargetFilterSummary {
	if !summary.Enabled {
		sort.Strings(summary.DisabledBy)
		summary.DisabledBy = compactStrings(summary.DisabledBy)
		summary.Fingerprint = ""
		return summary
	}
	for _, fqn := range summary.TargetFQNs {
		if name := simpleCalleeName(fqn); name != "" {
			summary.CalleeNames = append(summary.CalleeNames, name)
		}
	}
	sort.Strings(summary.CalleeNames)
	summary.CalleeNames = compactStrings(summary.CalleeNames)
	sort.Strings(summary.TargetFQNs)
	summary.TargetFQNs = compactStrings(summary.TargetFQNs)
	summary.Fingerprint = fingerprintCallTargetFilter(summary.CalleeNames, summary.TargetFQNs)
	return summary
}

// WriteCallTargetFilterFile writes the callee filter JSON consumed by
// krit-types --call-filter. Disabled summaries return an empty path.
func WriteCallTargetFilterFile(summary CallTargetFilterSummary, tmpDir string) (string, error) {
	if !summary.Enabled {
		return "", nil
	}
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	payload := callTargetFilterJSON{
		Version:     1,
		Mode:        "calleeNames",
		CalleeNames: append([]string(nil), summary.CalleeNames...),
		TargetFQNs:  append([]string(nil), summary.TargetFQNs...),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(tmpDir, "krit-call-filter-*.json")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	if _, err := f.Write([]byte("\n")); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func simpleCalleeName(fqn string) string {
	fqn = strings.TrimSpace(fqn)
	if fqn == "" {
		return ""
	}
	for _, sep := range []string{"#", "."} {
		if idx := strings.LastIndex(fqn, sep); idx >= 0 && idx < len(fqn)-1 {
			return fqn[idx+1:]
		}
	}
	return fqn
}

func fingerprintCallTargetFilter(calleeNames, targetFQNs []string) string {
	h := hashutil.Hasher().New()
	_, _ = h.Write([]byte("call-filter-v1\ncallee\n"))
	for _, name := range calleeNames {
		_, _ = h.Write([]byte(name))
		_, _ = h.Write([]byte{'\n'})
	}
	_, _ = h.Write([]byte("target\n"))
	for _, fqn := range targetFQNs {
		_, _ = h.Write([]byte(fqn))
		_, _ = h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil)[:8])
}

func compactStrings(in []string) []string {
	out := in[:0]
	var last string
	var haveLast bool
	for _, s := range in {
		if s == "" {
			continue
		}
		if haveLast && s == last {
			continue
		}
		out = append(out, s)
		last = s
		haveLast = true
	}
	return out
}

func removeCallTargetFilterFile(path string) {
	if path != "" {
		_ = os.Remove(filepath.Clean(path))
	}
}
