// Descriptor metadata for internal/rules/database.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *DaoNotInterfaceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DaoNotInterface",
		RuleSet:       "database",
		DefaultActive: false,
	}
}

func (r *DaoWithoutAnnotationsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DaoWithoutAnnotations",
		RuleSet:       "database",
		DefaultActive: true,
	}
}

func (r *EntityMutableColumnRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EntityMutableColumn",
		RuleSet:       "database",
		DefaultActive: false,
	}
}

func (r *DatabaseInstanceRecreatedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DatabaseInstanceRecreated",
		RuleSet:       "resource-cost",
		DefaultActive: true,
	}
}

func (r *ForeignKeyWithoutOnDeleteRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ForeignKeyWithoutOnDelete",
		RuleSet:       "database",
		DefaultActive: true,
	}
}

func (r *RoomConflictStrategyReplaceOnFkRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RoomConflictStrategyReplaceOnFk",
		RuleSet:       "database",
		DefaultActive: true,
	}
}

func (r *JdbcResultSetLeakedFromFunctionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "JdbcResultSetLeakedFromFunction",
		RuleSet:       "database",
		DefaultActive: true,
	}
}

func (r *EntityPrimaryKeyNotStableRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EntityPrimaryKeyNotStable",
		RuleSet:       "database",
		DefaultActive: true,
	}
}

func (r *RoomExportSchemaDisabledRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RoomExportSchemaDisabled",
		RuleSet:       "database",
		DefaultActive: true,
	}
}

func (r *RoomEntityChangedMigrationMissingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RoomEntityChangedMigrationMissing",
		RuleSet:       "database",
		DefaultActive: false,
	}
}

func (r *RoomRelationWithoutIndexRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RoomRelationWithoutIndex",
		RuleSet:       "database",
		DefaultActive: true,
	}
}

func (r *SqliteCursorWithoutCloseRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SqliteCursorWithoutClose",
		RuleSet:       "database",
		DefaultActive: true,
	}
}

func (r *RoomDatabaseVersionNotBumpedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RoomDatabaseVersionNotBumped",
		RuleSet:       "database",
		DefaultActive: false,
	}
}

func (r *RoomMultipleWritesMissingTransactionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RoomMultipleWritesMissingTransaction",
		RuleSet:       "database",
		DefaultActive: true,
	}
}

func (r *RoomMigrationUsesExecSQLWithInterpolationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RoomMigrationUsesExecSqlWithInterpolation",
		RuleSet:       "database",
		DefaultActive: true,
	}
}

func (r *RoomFallbackToDestructiveMigrationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RoomFallbackToDestructiveMigration",
		RuleSet:       "database",
		DefaultActive: true,
	}
}

func (r *RoomQueryMissingWhereForUpdateRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RoomQueryMissingWhereForUpdate",
		RuleSet:       "database",
		DefaultActive: true,
	}
}

func (r *RoomSelectStarWithoutLimitRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RoomSelectStarWithoutLimit",
		RuleSet:       "database",
		DefaultActive: true,
	}
}

func (r *RoomFlowQueryWithoutDistinctRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RoomFlowQueryWithoutDistinct",
		RuleSet:       "database",
		DefaultActive: false,
	}
}

func (r *RoomQueryWithLikeMissingEscapeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RoomQueryWithLikeMissingEscape",
		RuleSet:       "database",
		DefaultActive: false,
	}
}

func (r *RoomSuspendQueryInTransactionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RoomSuspendQueryInTransaction",
		RuleSet:       "database",
		DefaultActive: true,
	}
}

func (r *JdbcPreparedStatementNotClosedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "JdbcPreparedStatementNotClosed",
		RuleSet:       "database",
		DefaultActive: true,
	}
}
