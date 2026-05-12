package rules

import (
	"fmt"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// PrecompileDeprecatedSymbolUsedErrorRule flags references to symbols
// whose @Deprecated annotation specifies level = DeprecationLevel.ERROR.
// Mirrors kotlinc's DEPRECATION_ERROR.
//
// Library symbols are not yet classified by level: the oracle exposes
// kotlin.Deprecated FQNs but no annotation arguments, and firing on
// every @Deprecated would flood ordinary (default WARNING) call sites.
// The rule declares NeedsOracleMemberAnnotations so the oracle profile
// is already sized for the library path once the JVM tools surface the
// `level` argument.
type PrecompileDeprecatedSymbolUsedErrorRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *PrecompileDeprecatedSymbolUsedErrorRule) Confidence() float64 { return 0.95 }

func (r *PrecompileDeprecatedSymbolUsedErrorRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File

	deprecated := deprecatedDeclIndex(file)
	if len(deprecated) == 0 {
		return
	}

	nodeType := file.FlatType(idx)
	if file.FlatHasAncestorOfType(idx, "import_header") {
		return
	}
	if nodeType == "navigation_expression" {
		if parent, ok := file.FlatParent(idx); ok && file.FlatType(parent) == "call_expression" {
			return
		}
	}
	if nodeType == "user_type" && file.FlatHasAncestorOfType(idx, "annotation") {
		return
	}

	name := flatDeprecationRefName(file, idx)
	if name == "" {
		return
	}

	info := deprecated[name]
	if info == nil || info.level != "ERROR" {
		return
	}

	msg := fmt.Sprintf("'%s' is deprecated with level ERROR and must not be used.", name)
	if info.message != "" {
		msg = fmt.Sprintf("'%s' is deprecated with level ERROR: %s", name, info.message)
	}
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
}
