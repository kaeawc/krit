// Descriptor metadata for internal/rules/database.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *DaoNotInterfaceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DaoNotInterface",
		RuleSet:       "database",
		Severity:      "info",
		Description:   "Detects Room DAOs declared as abstract classes instead of interfaces.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DaoWithoutAnnotationsRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DaoWithoutAnnotations",
		RuleSet:       "database",
		Severity:      "warning",
		Description:   "Detects Room DAO member functions missing required annotations like @Query, @Insert, @Update, or @Delete.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *EntityMutableColumnRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EntityMutableColumn",
		RuleSet:       "database",
		Severity:      "info",
		Description:   "Detects Room @Entity class primary-constructor parameters declared as var, which prevents straightforward copy-on-write.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DatabaseInstanceRecreatedRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DatabaseInstanceRecreated",
		RuleSet:       "resource-cost",
		Severity:      "warning",
		Description:   "Detects Room.databaseBuilder calls inside regular functions that recreate the database on each invocation.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ForeignKeyWithoutOnDeleteRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ForeignKeyWithoutOnDelete",
		RuleSet:       "database",
		Severity:      "warning",
		Description:   "Detects Room @ForeignKey(...) without an onDelete argument; the default NO_ACTION usually leaves stale rows.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RoomConflictStrategyReplaceOnFkRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RoomConflictStrategyReplaceOnFk",
		RuleSet:       "database",
		Severity:      "warning",
		Description:   "Detects @Insert(onConflict = REPLACE) on a Room entity that declares foreign keys; REPLACE deletes and re-inserts, cascading FK deletes.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *JdbcResultSetLeakedFromFunctionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "JdbcResultSetLeakedFromFunction",
		RuleSet:       "database",
		Severity:      "warning",
		Description:   "Detects functions whose declared return type is java.sql.ResultSet; callers almost always forget to close the ResultSet.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *EntityPrimaryKeyNotStableRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EntityPrimaryKeyNotStable",
		RuleSet:       "database",
		Severity:      "warning",
		Description:   "Detects @Entity @PrimaryKey declared as var without autoGenerate = true; mutable primary keys break equals/hashCode.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RoomExportSchemaDisabledRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RoomExportSchemaDisabled",
		RuleSet:       "database",
		Severity:      "warning",
		Description:   "Detects Room @Database(exportSchema = false); disabling schema export loses migration history.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RoomEntityChangedMigrationMissingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RoomEntityChangedMigrationMissing",
		RuleSet:       "database",
		Severity:      "warning",
		Description:   "Detects @Entity columns whose names do not appear in any Room Migration(M, N) declaration in the project.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.6,
	}
}

func (r *RoomRelationWithoutIndexRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RoomRelationWithoutIndex",
		RuleSet:       "database",
		Severity:      "warning",
		Description:   "Detects @Relation(entityColumn = ...) referencing a column that is not declared in the target @Entity's indices.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SqliteCursorWithoutCloseRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SqliteCursorWithoutClose",
		RuleSet:       "database",
		Severity:      "warning",
		Description:   "Detects SQLiteDatabase rawQuery/query cursors assigned to local properties without .use {} or .close() in the same scope.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *JdbcPreparedStatementNotClosedRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "JdbcPreparedStatementNotClosed",
		RuleSet:       "database",
		Severity:      "warning",
		Description:   "Detects JDBC prepared statements assigned to local properties without .use {} or .close() in the same scope.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
