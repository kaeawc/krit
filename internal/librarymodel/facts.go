package librarymodel

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

// FingerprintVersion bumps the schema for Facts fingerprints. Bump this when
// adding fields whose absence would let stale cache entries silently apply.
const FingerprintVersion = 1

// Fingerprint returns a stable hex digest of the semantic content of f.
// Callers use this as part of cache keys for project-level analysis whose
// findings depend on library facts (Android phase, library-aware Kotlin
// rules). nil and zero-value Facts are distinguished from non-empty Facts
// by the version prefix.
func (f *Facts) Fingerprint() string {
	var buf []byte
	buf = append(buf, "librarymodel.Facts/v"...)
	buf = append(buf, byte(FingerprintVersion))
	if f == nil {
		buf = append(buf, "|nil"...)
		return hashutil.HashHex(buf)
	}
	// json.Marshal of structs is deterministic (field order fixed by
	// struct definition). Maps are sorted by encoding/json. The Facts
	// graph contains only structs, slices, primitives, and string maps,
	// so this is stable without a custom encoder.
	data, err := json.Marshal(f.relocatable())
	if err != nil {
		buf = append(buf, "|err:"...)
		buf = append(buf, err.Error()...)
		return hashutil.HashHex(buf)
	}
	if base := f.Profile.InputBase; base != "" {
		// Every JSON string naming a path under the build files' shared
		// directory (dependency sources, catalogs, source roots) is spelled
		// relative to it.
		if quoted, err := json.Marshal(base + string(filepath.Separator)); err == nil {
			data = bytes.ReplaceAll(data, quoted[:len(quoted)-1], []byte(`"./`))
		}
	}
	buf = append(buf, '|')
	buf = append(buf, data...)
	return hashutil.HashHex(buf)
}

// relocatable returns f with the paths in its Java source profile spelled
// relative to the directory they share (the source roots together, each
// classpath list on its own), so the fingerprint stays the same when the
// whole checkout, or GRADLE_USER_HOME, moves to another absolute path
// (#1662). f is not modified.
func (f *Facts) relocatable() *Facts {
	j := f.Profile.Java
	sources := [][]string{j.SourceRoots, j.TestSourceRoots, j.AndroidTestSourceRoots, j.GeneratedSourceRoots}
	var all []string
	for _, list := range sources {
		all = append(all, list...)
	}
	if len(all) == 0 && len(j.ClasspathCandidates) == 0 && len(j.BootClasspathCandidates) == 0 {
		if f.Profile.InputBase == "" {
			return f
		}
		out := *f
		out.Profile.InputBase = ""
		return &out
	}
	rel := func(base string, paths []string) []string {
		if base == "" || len(paths) == 0 {
			return paths
		}
		out := make([]string, len(paths))
		for i, p := range paths {
			out[i] = fsutil.RelSlash(base, p)
		}
		return out
	}
	base := fsutil.CommonDir(all)
	out := *f
	out.Profile.InputBase = "" // Fingerprint relativizes against it
	out.Profile.Java.SourceRoots = rel(base, j.SourceRoots)
	out.Profile.Java.TestSourceRoots = rel(base, j.TestSourceRoots)
	out.Profile.Java.AndroidTestSourceRoots = rel(base, j.AndroidTestSourceRoots)
	out.Profile.Java.GeneratedSourceRoots = rel(base, j.GeneratedSourceRoots)
	out.Profile.Java.ClasspathCandidates = rel(classpathBase(j.ClasspathCandidates), j.ClasspathCandidates)
	out.Profile.Java.BootClasspathCandidates = rel(classpathBase(j.BootClasspathCandidates), j.BootClasspathCandidates)
	return &out
}

// Facts is the version/profile-aware semantic model consumed by rules.
type Facts struct {
	Profile  ProjectProfile
	Database DatabaseFacts
	Java     JavaTypeProfile
}

type DatabaseFacts struct {
	SQLite     SQLiteFacts
	Room       RoomFacts
	SQLDelight SQLDelightFacts
}

type SQLiteFacts struct {
	BlockingMethods []string
}

type RoomFacts struct {
	Enabled              bool
	DaoAnnotations       []string
	OperationAnnotations []string
	AsyncReturnTypeHints []string
	LoadAllMethodNames   []string
	LoadOneTerminalNames []string
	QueryAnnotationNames []string
}

type SQLDelightFacts struct {
	Enabled                  bool
	BlockingExecutionMethods []string
}

// DefaultFacts returns conservative built-in facts for contexts where Krit has
// no project profile. This preserves behavior for single-file editor/LSP use
// and unit tests while allowing repository scans to narrow facts from Gradle.
func DefaultFacts() *Facts {
	return &Facts{
		Java: DefaultJavaTypeProfile(),
		Database: DatabaseFacts{
			SQLite: SQLiteFacts{
				BlockingMethods: []string{"rawQuery", "query", "execSQL"},
			},
			Room: RoomFacts{
				Enabled:              true,
				DaoAnnotations:       []string{"Dao", "androidx.room.Dao"},
				OperationAnnotations: []string{"Query", "Insert", "Update", "Delete", "Transaction", "androidx.room.Query", "androidx.room.Insert", "androidx.room.Update", "androidx.room.Delete", "androidx.room.Transaction"},
				QueryAnnotationNames: []string{"Query", "androidx.room.Query"},
				AsyncReturnTypeHints: []string{"Flow<", "LiveData<", "PagingSource<", "DataSource.Factory<"},
				LoadAllMethodNames:   []string{"getAll", "findAll", "loadAll", "fetchAll", "queryAll", "selectAll"},
				LoadOneTerminalNames: []string{"first", "firstOrNull", "single", "singleOrNull", "last", "lastOrNull"},
			},
			SQLDelight: SQLDelightFacts{
				Enabled:                  true,
				BlockingExecutionMethods: []string{"executeAsList", "executeAsOne", "executeAsOneOrNull", "executeAsOptional", "executeAsCursor"},
			},
		},
	}
}

// FactsForProfile builds library facts from a project profile.
func FactsForProfile(profile ProjectProfile) *Facts {
	facts := DefaultFacts()
	facts.Profile = profile
	facts.Database.Room.Enabled = roomApplies(profile)
	facts.Database.SQLDelight.Enabled = sqlDelightApplies(profile)
	facts.Java = javaTypeProfile(facts.Database.Room.Enabled)
	return facts
}

func EnsureFacts(facts *Facts) *Facts {
	if facts != nil {
		return facts
	}
	return DefaultFacts()
}

func roomApplies(profile ProjectProfile) bool {
	return profile.MayUseAnyDependency(
		Coordinate{Group: "androidx.room", Name: "room-runtime"},
		Coordinate{Group: "androidx.room", Name: "room-ktx"},
		Coordinate{Group: "androidx.room", Name: "room-common"},
		Coordinate{Group: "androidx.room", Name: "room-compiler"},
	)
}

func sqlDelightApplies(profile ProjectProfile) bool {
	return profile.MayUseAnyDependency(
		Coordinate{Group: "app.cash.sqldelight", Name: "runtime"},
		Coordinate{Group: "app.cash.sqldelight", Name: "android-driver"},
		Coordinate{Group: "app.cash.sqldelight", Name: "coroutines-extensions"},
		Coordinate{Group: "com.squareup.sqldelight", Name: "runtime"},
		Coordinate{Group: "com.squareup.sqldelight", Name: "android-driver"},
		Coordinate{Group: "com.squareup.sqldelight", Name: "coroutines-extensions"},
	)
}

func (f *Facts) DatabaseFacts() DatabaseFacts {
	return EnsureFacts(f).Database
}

func (d DatabaseFacts) IsSQLiteBlockingMethod(name string) bool {
	return stringIn(name, d.SQLite.BlockingMethods)
}

func (r RoomFacts) HasDaoAnnotation(name string) bool {
	return r.Enabled && stringIn(name, r.DaoAnnotations)
}

func (r RoomFacts) HasOperationAnnotation(name string) bool {
	return r.Enabled && stringIn(name, r.OperationAnnotations)
}

func (r RoomFacts) HasQueryAnnotation(name string) bool {
	return r.Enabled && stringIn(name, r.QueryAnnotationNames)
}

func (r RoomFacts) IsAsyncReturnType(header string) bool {
	if !r.Enabled {
		return false
	}
	for _, hint := range r.AsyncReturnTypeHints {
		if strings.Contains(header, hint) {
			return true
		}
	}
	return false
}

func (r RoomFacts) IsLoadAllMethod(name string) bool {
	return r.Enabled && stringIn(name, r.LoadAllMethodNames)
}

func (r RoomFacts) IsLoadOneTerminal(name string) bool {
	return r.Enabled && stringIn(name, r.LoadOneTerminalNames)
}

func (s SQLDelightFacts) IsBlockingExecutionMethod(name string) bool {
	return s.Enabled && stringIn(name, s.BlockingExecutionMethods)
}

func stringIn(value string, values []string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

// classpathBase is the directory a classpath list is spelled relative to:
// the entries' common directory, one level higher when an entry sits right in
// it. Each jar then keeps at least its own folder, which in a Gradle cache is
// the jar's content hash; a one-jar list would otherwise collapse to the bare
// file name, and another version of that jar would fingerprint the same.
func classpathBase(paths []string) string {
	base := fsutil.CommonDir(paths)
	if base == "" {
		return ""
	}
	for _, p := range paths {
		if filepath.Dir(filepath.Clean(p)) == base {
			return filepath.Dir(base)
		}
	}
	return base
}
