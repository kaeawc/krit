package oracle

import (
	"fmt"

	"github.com/kaeawc/krit/internal/typeinfer"
)

// FakeOracle is a configurable test double for the Lookup interface.
// Set up responses before using in tests.
type FakeOracle struct {
	Classes               map[string]*typeinfer.ClassInfo
	Sealed                map[string][]string
	Enums                 map[string][]string
	Subtypes              map[string][]string // type → supertypes (for IsSubtype)
	Functions             map[string]*typeinfer.ResolvedType
	Deps                  map[string]*OracleClass
	Expressions           map[string]map[string]*typeinfer.ResolvedType // file → ("line:col" → type)
	Annotations           map[string][]string                           // "ClassName.memberName" → annotation FQNs
	CallTargets           map[string]map[string]string                  // file → ("line:col" → call target FQN)
	CallTargetSuspend     map[string]map[string]bool                    // file → ("line:col" → suspend status for resolved call target)
	CallTargetAnnotations map[string]map[string][]string                // file → ("line:col" → annotation FQNs on symbol)
	Diagnostics           map[string][]OracleDiagnostic                 // file → diagnostics
}

// NewFakeOracle creates a FakeOracle with all maps initialized.
func NewFakeOracle() *FakeOracle {
	return &FakeOracle{
		Classes:               make(map[string]*typeinfer.ClassInfo),
		Sealed:                make(map[string][]string),
		Enums:                 make(map[string][]string),
		Subtypes:              make(map[string][]string),
		Functions:             make(map[string]*typeinfer.ResolvedType),
		Deps:                  make(map[string]*OracleClass),
		Expressions:           make(map[string]map[string]*typeinfer.ResolvedType),
		Annotations:           make(map[string][]string),
		CallTargets:           make(map[string]map[string]string),
		CallTargetSuspend:     make(map[string]map[string]bool),
		CallTargetAnnotations: make(map[string]map[string][]string),
		Diagnostics:           make(map[string][]OracleDiagnostic),
	}
}

func (f *FakeOracle) LookupClass(name string) *typeinfer.ClassInfo {
	if info, ok := f.Classes[name]; ok {
		return info
	}
	return nil
}

func (f *FakeOracle) LookupSealedVariants(name string) []string {
	return f.Sealed[name]
}

func (f *FakeOracle) LookupEnumEntries(name string) []string {
	return f.Enums[name]
}

func (f *FakeOracle) IsSubtype(a, b string) bool {
	if a == b {
		return true
	}
	for _, st := range f.Subtypes[a] {
		if st == b {
			return true
		}
	}
	return false
}

func (f *FakeOracle) Dependencies() map[string]*OracleClass {
	return f.Deps
}

func (f *FakeOracle) LookupFunction(key string) *typeinfer.ResolvedType {
	return f.Functions[key]
}

func (f *FakeOracle) LookupExpression(filePath string, line, col int) *typeinfer.ResolvedType {
	fileExprs := f.Expressions[filePath]
	if fileExprs == nil {
		return nil
	}
	key := fmt.Sprintf("%d:%d", line, col)
	return fileExprs[key]
}

func (f *FakeOracle) LookupAnnotations(key string) []string {
	return f.Annotations[key]
}

func (f *FakeOracle) LookupCallTarget(filePath string, line, col int) string {
	fileCTs := f.CallTargets[filePath]
	if fileCTs == nil {
		return ""
	}
	key := fmt.Sprintf("%d:%d", line, col)
	return fileCTs[key]
}

func (f *FakeOracle) LookupCallTargetSuspend(filePath string, line, col int) (bool, bool) {
	fileCTS := f.CallTargetSuspend[filePath]
	if fileCTS == nil {
		return false, false
	}
	key := fmt.Sprintf("%d:%d", line, col)
	isSuspend, ok := fileCTS[key]
	return isSuspend, ok
}

func (f *FakeOracle) LookupCallTargetAnnotations(filePath string, line, col int) []string {
	fileCTAs := f.CallTargetAnnotations[filePath]
	if fileCTAs == nil {
		return nil
	}
	key := fmt.Sprintf("%d:%d", line, col)
	return fileCTAs[key]
}

func (f *FakeOracle) LookupDiagnostics(filePath string) []OracleDiagnostic {
	return f.Diagnostics[filePath]
}

// Compile-time check that FakeOracle implements Lookup.
var _ Lookup = (*FakeOracle)(nil)
