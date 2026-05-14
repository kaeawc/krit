// Descriptor metadata for internal/rules/security.go.

package rules

import (
	"regexp"

	api "github.com/kaeawc/krit/internal/rules/api"
)

func (r *ContentProviderQueryWithSelectionInterpolationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ContentProviderQueryWithSelectionInterpolation",
		RuleSet:       "security",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *SQLInjectionRawQueryRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SqlInjectionRawQuery",
		RuleSet:       "security",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *RuntimeExecUnsafeShapeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RuntimeExecUnsafeShape",
		RuleSet:       "security",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *RoomRawQueryStringConcatRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RoomRawQueryStringConcat",
		RuleSet:       "security",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *ProcessBuilderShellArgRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ProcessBuilderShellArg",
		RuleSet:       "security",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *LogPiiRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LogPii",
		RuleSet:       "security",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[LogPiiRule]{
				Name:        "piiNamePattern",
				Default:     defaultLogPiiNamePattern.String(),
				Description: "Regex for variable names that should be treated as sensitive when logged.",
				Apply:       func(r *LogPiiRule, v *regexp.Regexp) { r.PiiNamePattern = v },
			}),
		},
	}
}

func (r *JdbcStatementExecuteRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "JdbcStatementExecute",
		RuleSet:       "security",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *XMLExternalEntityRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "XmlExternalEntity",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *JavaObjectInputStreamRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "JavaObjectInputStream",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *JacksonDefaultTypingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "JacksonDefaultTyping",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *GsonPolymorphicFromJSONRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GsonPolymorphicFromJson",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *FileFromUntrustedPathRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "FileFromUntrustedPath",
		RuleSet:       "security",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *HardcodedBearerTokenRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HardcodedBearerToken",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *HardcodedGcpServiceAccountRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HardcodedGcpServiceAccount",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *HardcodedJwtRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HardcodedJwt",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *HardcodedAwsAccessKeyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HardcodedAwsAccessKey",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *ZipSlipUncheckedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ZipSlipUnchecked",
		RuleSet:       "security",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *TempFileWorldReadableRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TempFileWorldReadable",
		RuleSet:       "security",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}
