package librarymodel

import "strings"

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
