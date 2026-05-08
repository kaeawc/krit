// Descriptor metadata for internal/rules/android_correctness.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *AssertRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "Assert",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *CheckResultRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CheckResult",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *CommitPrefEditsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CommitPrefEdits",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *CommitTransactionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CommitTransaction",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *DefaultLocaleRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DefaultLocale",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *NestedScrollingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NestedScrolling",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *RegisteredRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "Registered",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *SQLiteStringRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SQLiteString",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *ScrollViewCountRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ScrollViewCount",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *SetTextI18nRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SetTextI18n",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *ShiftFlagsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ShiftFlags",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *SimpleDateFormatRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SimpleDateFormat",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *StopShipRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StopShip",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *UniqueConstantsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UniqueConstants",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *WrongCallRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WrongCall",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *WrongThreadRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WrongThread",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}
