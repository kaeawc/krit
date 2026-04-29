// Descriptor metadata for internal/rules/database.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *DaoNotInterfaceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "DaoNotInterface",
		RuleSet:       "database",
		Severity:      "info",
		Description:   "Detects Room DAOs declared as abstract classes instead of interfaces.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DaoWithoutAnnotationsRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "DaoWithoutAnnotations",
		RuleSet:       "database",
		Severity:      "warning",
		Description:   "Detects Room DAO member functions missing required annotations like @Query, @Insert, @Update, or @Delete.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DatabaseInstanceRecreatedRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "DatabaseInstanceRecreated",
		RuleSet:       "resource-cost",
		Severity:      "warning",
		Description:   "Detects Room.databaseBuilder calls inside regular functions that recreate the database on each invocation.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ForeignKeyWithoutOnDeleteRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ForeignKeyWithoutOnDelete",
		RuleSet:       "database",
		Severity:      "warning",
		Description:   "Detects Room @ForeignKey(...) without an onDelete argument; the default NO_ACTION usually leaves stale rows.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *JdbcPreparedStatementNotClosedRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "JdbcPreparedStatementNotClosed",
		RuleSet:       "database",
		Severity:      "warning",
		Description:   "Detects JDBC prepared statements assigned to local properties without .use {} or .close() in the same scope.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
