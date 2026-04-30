// Package registry provides compatibility aliases for rule metadata.
//
// Metadata ownership lives in internal/rules/v2. This package remains so the
// existing generated Meta files and tests can move to v2 in smaller PRs.
package registry

import v2 "github.com/kaeawc/krit/internal/rules/v2"

type RuleDescriptor = v2.RuleDescriptor
type ConfigOption = v2.ConfigOption
type OptionType = v2.OptionType

const (
	OptInt        = v2.OptInt
	OptBool       = v2.OptBool
	OptString     = v2.OptString
	OptStringList = v2.OptStringList
	OptRegex      = v2.OptRegex
)

type MetaProvider = v2.MetaProvider
