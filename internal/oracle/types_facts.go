package oracle

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/buildid"
	"github.com/kaeawc/krit/internal/fsutil"
)

// typesFactsRecord describes the compile scope and Krit build a cached
// types.json was produced under. The freshness gate reuses types.json
// wholesale when no source changed, so without this record a types.json
// computed under other settings (backend, jar, classpath, Kotlin settings,
// or a different file-key spelling) would be served to a later run.
//
// The record is a sidecar rather than a types.json field because the JVM
// one-shot path writes types.json itself. TypesSize/TypesModNanos pin the
// record to the exact file it describes: if any other writer replaces
// types.json, the record no longer matches and is ignored.
type typesFactsRecord struct {
	TypesSize      int64  `json:"typesSize"`
	TypesModNanos  int64  `json:"typesModNanos"`
	Backend        string `json:"backend"`
	JarToken       string `json:"jarToken"`
	ClasspathToken string `json:"classpathToken"`
	BuildToken     string `json:"buildToken"`
	// ScopeVersion is the store scope's identity hash: it covers the backend,
	// jar, classpath, JVM target and Kotlin compiler settings, so a
	// settings-only change cannot reuse a types.json computed under others.
	ScopeVersion string `json:"scopeVersion,omitempty"`
	// KeyBase pins the spelling of the file keys inside types.json: empty
	// when the scan used absolute roots, the working directory when it used
	// relative ones (`krit .`), whose keys are relative to it. A run with a
	// different base cannot look its absolute (or differently relative) paths
	// up in those keys and must not reuse the file.
	KeyBase string `json:"keyBase,omitempty"`
}

// KeyBaseFor returns the file-key spelling token for a scan over scanPaths:
// "" when every scan path is absolute, else the working directory.
func KeyBaseFor(scanPaths []string) string {
	for _, p := range scanPaths {
		if p != "" && !filepath.IsAbs(p) {
			if wd, err := os.Getwd(); err == nil {
				return wd
			}
			return "?"
		}
	}
	return ""
}

func typesFactsPath(typesPath string) string { return typesPath + ".facts" }

// RecordTypesFacts notes the scope and build the types.json at typesPath was
// produced under. Call it after the oracle wrote the file.
func RecordTypesFacts(typesPath string, scope ...StoreScope) error {
	return RecordTypesFactsForPaths(typesPath, nil, scope...)
}

// RecordTypesFactsForPaths is RecordTypesFacts that also records the key
// spelling of the scan over scanPaths.
func RecordTypesFactsForPaths(typesPath string, scanPaths []string, scope ...StoreScope) error {
	info, err := os.Stat(typesPath)
	if err != nil {
		return err
	}
	rec := typesFactsRecord{
		TypesSize:     info.Size(),
		TypesModNanos: info.ModTime().UnixNano(),
		BuildToken:    buildid.Token(),
		KeyBase:       KeyBaseFor(scanPaths),
	}
	if len(scope) > 0 {
		rec.Backend = scope[0].Backend.String()
		rec.JarToken = scope[0].JarToken
		rec.ClasspathToken = scope[0].ClasspathToken
		rec.ScopeVersion = scope[0].versionHex()
	}
	body, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(typesFactsPath(typesPath), body, 0o644)
}

// TypesJSONSatisfies checks the exact backend, jar, compile settings, and
// Krit build that produced a cached types.json. A missing, unreadable, or
// stale record (types.json replaced since it was written) reports false, so
// the caller re-runs the oracle rather than trusting facts it cannot vouch
// for. krit-fir always collects its retained compiler diagnostics, so every
// recorded types.json carries them.
func TypesJSONSatisfies(typesPath string, scope StoreScope, buildToken string) bool {
	return TypesJSONSatisfiesForPaths(typesPath, scope, buildToken, nil)
}

// TypesJSONSatisfiesForPaths additionally requires the file keys to be spelled
// the way a scan over scanPaths looks them up.
func TypesJSONSatisfiesForPaths(typesPath string, scope StoreScope, buildToken string, scanPaths []string) bool {
	rec, ok := readTypesFacts(typesPath)
	return ok && rec.KeyBase == KeyBaseFor(scanPaths) && rec.Backend == scope.Backend.String() && rec.JarToken == scope.JarToken && rec.ClasspathToken == scope.ClasspathToken &&
		rec.ScopeVersion == scope.versionHex() && rec.BuildToken == buildToken
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
