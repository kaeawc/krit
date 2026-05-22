package coroutines

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/base"
	"github.com/kaeawc/krit/internal/scanner"
)

// SuspendFunWithFlowReturnTypeRule flags `suspend fun foo(): Flow<T>` and
// related Flow-returning suspend functions. Flow builders are cold and do
// not require the caller to be in a coroutine, so the suspend modifier is
// redundant at best and misleading at worst.
type SuspendFunWithFlowReturnTypeRule struct {
	base.FlatDispatchBase
	base.BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Coroutines rule.
func (r *SuspendFunWithFlowReturnTypeRule) Confidence() float64 { return api.ConfidenceHigh }

// IsFixable marks this rule as auto-fixable: stripping the suspend modifier.
func (r *SuspendFunWithFlowReturnTypeRule) IsFixable() bool { return true }

// Meta returns the descriptor metadata for the rule.
func (r *SuspendFunWithFlowReturnTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SuspendFunWithFlowReturnType",
		RuleSet:       "coroutines",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

// flowReturnTypeNames is the recognized set of cold-Flow types whose
// suspending wrapper is structurally redundant.
var flowReturnTypeNames = map[string]bool{
	"Flow":              true,
	"StateFlow":         true,
	"SharedFlow":        true,
	"MutableStateFlow":  true,
	"MutableSharedFlow": true,
}

func init() {
	r := &SuspendFunWithFlowReturnTypeRule{
		BaseRule: base.BaseRule{
			RuleName:    "SuspendFunWithFlowReturnType",
			RuleSetName: "coroutines",
			Sev:         "warning",
			Desc:        "Detects suspend functions that return a Flow type, since Flow builders are cold and do not require suspend.",
		},
	}
	api.Register(&api.Rule{
		ID:             r.RuleName,
		Category:       r.RuleSetName,
		Description:    r.Desc,
		Sev:            api.Severity(r.Sev),
		NodeTypes:      []string{"function_declaration"},
		Confidence:     0.85,
		Fix:            api.FixIdiomatic,
		Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !file.FlatHasModifier(idx, "suspend") {
				return
			}
			userType, ok := file.FlatFindChild(idx, "user_type")
			if !ok {
				return
			}
			typeIdent, ok := file.FlatFindChild(userType, "type_identifier")
			if !ok || !flowReturnTypeNames[file.FlatNodeText(typeIdent)] {
				return
			}
			f := r.Finding(file, file.FlatRow(idx)+1, 1,
				"Suspend function returns a Flow type. A function that returns Flow should not be suspend. The flow builder is cold and does not require a coroutine.")
			suspendNode := file.FlatFindModifierNode(idx, "suspend")
			if suspendNode != 0 {
				endByte := int(file.FlatEndByte(suspendNode))
				if endByte < len(file.Content) && file.Content[endByte] == ' ' {
					endByte++
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(suspendNode)),
					EndByte:     endByte,
					Replacement: "",
				}
			}
			ctx.Emit(f)
		},
	})
}
