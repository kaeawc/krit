package oracle

import (
	"encoding/json"
	"os"

	"github.com/kaeawc/krit/internal/fsutil"
)

// typesFactsRecord describes which optional facts a cached types.json
// holds. The freshness gate reuses types.json wholesale when no source
// changed, so without this record a types.json written with diagnostics
// disabled (no projection rule active) would be served to a later run whose
// rules consume diagnostics, silently dropping every projected finding.
//
// The record is a sidecar rather than a types.json field because the JVM
// one-shot path writes types.json itself. TypesSize/TypesModNanos pin the
// record to the exact file it describes: if any other writer replaces
// types.json, the record no longer matches and is ignored.
type typesFactsRecord struct {
	DiagnosticsOmitted bool  `json:"diagnosticsOmitted"`
	TypesSize          int64 `json:"typesSize"`
	TypesModNanos      int64 `json:"typesModNanos"`
}

func typesFactsPath(typesPath string) string { return typesPath + ".facts" }

// RecordTypesFacts notes whether the types.json at typesPath was produced
// with compiler diagnostics omitted. Call it after the oracle wrote the file.
func RecordTypesFacts(typesPath string, diagnosticsOmitted bool) error {
	info, err := os.Stat(typesPath)
	if err != nil {
		return err
	}
	body, err := json.Marshal(typesFactsRecord{
		DiagnosticsOmitted: diagnosticsOmitted,
		TypesSize:          info.Size(),
		TypesModNanos:      info.ModTime().UnixNano(),
	})
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(typesFactsPath(typesPath), body, 0o644)
}

// TypesJSONHasDiagnostics reports whether the types.json at typesPath is
// known to include compiler diagnostics. A missing, unreadable, or stale
// record (types.json replaced since it was written) reports false, so the
// caller re-runs the oracle rather than trusting facts it cannot vouch for.
func TypesJSONHasDiagnostics(typesPath string) bool {
	info, err := os.Stat(typesPath)
	if err != nil {
		return false
	}
	body, err := os.ReadFile(typesFactsPath(typesPath))
	if err != nil {
		return false
	}
	var rec typesFactsRecord
	if json.Unmarshal(body, &rec) != nil {
		return false
	}
	if rec.TypesSize != info.Size() || rec.TypesModNanos != info.ModTime().UnixNano() {
		return false
	}
	return !rec.DiagnosticsOmitted
}
