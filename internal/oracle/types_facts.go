package oracle

import (
	"encoding/json"
	"os"

	"github.com/kaeawc/krit/internal/buildid"
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
	DiagnosticsOmitted bool   `json:"diagnosticsOmitted"`
	TypesSize          int64  `json:"typesSize"`
	TypesModNanos      int64  `json:"typesModNanos"`
	Backend            string `json:"backend"`
	JarToken           string `json:"jarToken"`
	BuildToken         string `json:"buildToken"`
}

func typesFactsPath(typesPath string) string { return typesPath + ".facts" }

// RecordTypesFacts notes whether the types.json at typesPath was produced
// with compiler diagnostics omitted. Call it after the oracle wrote the file.
func RecordTypesFacts(typesPath string, diagnosticsOmitted bool, scope ...StoreScope) error {
	info, err := os.Stat(typesPath)
	if err != nil {
		return err
	}
	rec := typesFactsRecord{
		DiagnosticsOmitted: diagnosticsOmitted,
		TypesSize:          info.Size(),
		TypesModNanos:      info.ModTime().UnixNano(),
		BuildToken:         buildid.Token(),
	}
	if len(scope) > 0 {
		rec.Backend = scope[0].Backend.String()
		rec.JarToken = scope[0].JarToken
	}
	body, err := json.Marshal(rec)
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
	rec, ok := readTypesFacts(typesPath)
	return ok && !rec.DiagnosticsOmitted
}

// TypesJSONSatisfies checks both optional diagnostics and the exact backend,
// jar, and Krit build that produced a cached types.json.
func TypesJSONSatisfies(typesPath string, diagnosticsRequired bool, scope StoreScope, buildToken string) bool {
	rec, ok := readTypesFacts(typesPath)
	return ok && rec.Backend == scope.Backend.String() && rec.JarToken == scope.JarToken &&
		rec.BuildToken == buildToken && (!diagnosticsRequired || !rec.DiagnosticsOmitted)
}

func readTypesFacts(typesPath string) (typesFactsRecord, bool) {
	info, err := os.Stat(typesPath)
	if err != nil {
		return typesFactsRecord{}, false
	}
	body, err := os.ReadFile(typesFactsPath(typesPath))
	if err != nil {
		return typesFactsRecord{}, false
	}
	var rec typesFactsRecord
	if json.Unmarshal(body, &rec) != nil {
		return typesFactsRecord{}, false
	}
	if rec.TypesSize != info.Size() || rec.TypesModNanos != info.ModTime().UnixNano() {
		return typesFactsRecord{}, false
	}
	return rec, true
}
