package oracletest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// FakeOracleBuilder produces a *oracle.FakeOracle populated from spec.
// Use it as the Builder argument to RunContract for the fake.
func FakeOracleBuilder(t *testing.T, spec Spec) oracle.Lookup {
	t.Helper()

	f := oracle.NewFakeOracle()

	for fqn, info := range spec.Classes {
		f.Classes[fqn] = info
	}
	for parent, variants := range spec.SealedVariants {
		f.Sealed[parent] = append([]string(nil), variants...)
	}
	for enumFQN, entries := range spec.EnumEntries {
		f.Enums[enumFQN] = append([]string(nil), entries...)
	}
	for sub, supers := range spec.DirectSupertypes {
		f.Subtypes[sub] = append([]string(nil), supers...)
	}
	for key, typ := range spec.Functions {
		f.Functions[key] = typ
	}
	for key, anns := range spec.Annotations {
		f.Annotations[key] = append([]string(nil), anns...)
	}
	for _, p := range spec.Positions {
		key := fmt.Sprintf("%d:%d", p.Line, p.Col)
		if p.Type != nil {
			if f.Expressions[p.File] == nil {
				f.Expressions[p.File] = make(map[string]*typeinfer.ResolvedType)
			}
			f.Expressions[p.File][key] = p.Type
		}
		if p.CallTarget != "" {
			if f.CallTargets[p.File] == nil {
				f.CallTargets[p.File] = make(map[string]string)
			}
			f.CallTargets[p.File][key] = p.CallTarget
		}
		if p.Suspend != nil {
			if f.CallTargetSuspend[p.File] == nil {
				f.CallTargetSuspend[p.File] = make(map[string]bool)
			}
			f.CallTargetSuspend[p.File][key] = *p.Suspend
		}
		if len(p.Annotations) > 0 {
			if f.CallTargetAnnotations[p.File] == nil {
				f.CallTargetAnnotations[p.File] = make(map[string][]string)
			}
			f.CallTargetAnnotations[p.File][key] = append([]string(nil), p.Annotations...)
		}
	}
	for file, diags := range spec.Diagnostics {
		f.Diagnostics[file] = append([]oracle.Diagnostic(nil), diags...)
	}
	return f
}

// buildDepsSealedVariants inserts synthetic class entries for each sealed variant.
func buildDepsSealedVariants(deps map[string]*oracle.Class, sealedVariants map[string][]string) {
	for parentFQN, variants := range sealedVariants {
		pkg := packageOf(parentFQN)
		for _, v := range variants {
			variantFQN := joinPackage(pkg, v)
			if _, exists := deps[variantFQN]; !exists {
				deps[variantFQN] = &oracle.Class{
					FQN:        variantFQN,
					Kind:       "class",
					Supertypes: []string{parentFQN},
				}
			} else {
				deps[variantFQN].Supertypes = append(deps[variantFQN].Supertypes, parentFQN)
			}
		}
	}
}

// buildDepsEnumEntries inserts synthetic enum class entries.
func buildDepsEnumEntries(deps map[string]*oracle.Class, enumEntries map[string][]string) {
	for enumFQN, entries := range enumEntries {
		members := make([]*oracle.Member, 0, len(entries))
		for _, e := range entries {
			members = append(members, &oracle.Member{Name: e, Kind: "enum_entry"})
		}
		if cls, ok := deps[enumFQN]; ok {
			cls.Kind = "enum"
			cls.Members = append(cls.Members, members...)
		} else {
			deps[enumFQN] = &oracle.Class{FQN: enumFQN, Kind: "enum", Members: members}
		}
	}
}

// buildDepsDirectSupertypes ensures child classes are declared with their direct supertypes.
func buildDepsDirectSupertypes(deps map[string]*oracle.Class, directSupertypes map[string][]string) {
	for childFQN, supers := range directSupertypes {
		if cls, ok := deps[childFQN]; ok {
			cls.Supertypes = append(cls.Supertypes, supers...)
		} else {
			deps[childFQN] = &oracle.Class{
				FQN:        childFQN,
				Kind:       "class",
				Supertypes: append([]string(nil), supers...),
			}
		}
	}
}

// buildDepsFunctions adds function members to dep classes from a key→type map.
func buildDepsFunctions(t *testing.T, deps map[string]*oracle.Class, functions map[string]*typeinfer.ResolvedType) {
	t.Helper()
	for key, typ := range functions {
		ownerFQN, member, ok := splitOwnerMember(key)
		if !ok {
			t.Fatalf("LoadFromDataBuilder: function key %q must contain '.'", key)
		}
		retFQN := ""
		nullable := false
		if typ != nil {
			retFQN = typ.Name
			nullable = typ.Nullable
		}
		appendMember(deps, ownerFQN, &oracle.Member{
			Name:       member,
			Kind:       "function",
			ReturnType: retFQN,
			Nullable:   nullable,
		})
	}
}

// buildDepsAnnotations adds annotation data to dep classes from a key→anns map.
func buildDepsAnnotations(deps map[string]*oracle.Class, annotations map[string][]string) {
	for key, anns := range annotations {
		if ownerFQN, member, ok := splitOwnerMember(key); ok {
			appendMember(deps, ownerFQN, &oracle.Member{
				Name:        member,
				Kind:        "function",
				Annotations: append([]string(nil), anns...),
			})
			continue
		}
		if cls, exists := deps[key]; exists {
			cls.Annotations = append(cls.Annotations, anns...)
		} else {
			deps[key] = &oracle.Class{
				FQN:         key,
				Kind:        "class",
				Annotations: append([]string(nil), anns...),
			}
		}
	}
}

// buildFilesPositions populates file expression/call-target/suspend/annotation entries.
func buildFilesPositions(files map[string]*oracle.File, positions []PositionFact) {
	for _, p := range positions {
		f, ok := files[p.File]
		if !ok {
			f = &oracle.File{Expressions: make(map[string]*oracle.ExpressionType)}
			files[p.File] = f
		}
		key := fmt.Sprintf("%d:%d", p.Line, p.Col)
		et, exists := f.Expressions[key]
		if !exists {
			et = &oracle.ExpressionType{}
			f.Expressions[key] = et
		}
		if p.Type != nil {
			et.Type = p.Type.Name
			et.Nullable = p.Type.Nullable
		}
		if p.CallTarget != "" {
			et.CallTarget = p.CallTarget
		}
		if p.Suspend != nil {
			et.CallTargetResolved = true
			et.CallTargetSuspend = *p.Suspend
		}
		if len(p.Annotations) > 0 {
			et.Annotations = append([]string(nil), p.Annotations...)
		}
	}
}

// buildFilesDiagnostics populates per-file diagnostics entries.
func buildFilesDiagnostics(files map[string]*oracle.File, diagnostics map[string][]oracle.Diagnostic) {
	for path, diags := range diagnostics {
		f, ok := files[path]
		if !ok {
			f = &oracle.File{}
			files[path] = f
		}
		for i := range diags {
			d := diags[i]
			f.Diagnostics = append(f.Diagnostics, &d)
		}
	}
}

// LoadFromDataBuilder produces a *oracle.Oracle populated from spec by
// synthesizing an Data and feeding it through oracle.LoadFromData.
// Use it as the Builder argument to RunContract for the real oracle.
func LoadFromDataBuilder(t *testing.T, spec Spec) oracle.Lookup {
	t.Helper()

	deps := map[string]*oracle.Class{}
	files := map[string]*oracle.File{}

	for fqn, info := range spec.Classes {
		deps[fqn] = &oracle.Class{
			FQN:        fqn,
			Kind:       info.Kind,
			Supertypes: append([]string(nil), info.Supertypes...),
		}
	}

	buildDepsSealedVariants(deps, spec.SealedVariants)
	buildDepsEnumEntries(deps, spec.EnumEntries)
	buildDepsDirectSupertypes(deps, spec.DirectSupertypes)
	buildDepsFunctions(t, deps, spec.Functions)
	buildDepsAnnotations(deps, spec.Annotations)
	buildFilesPositions(files, spec.Positions)
	buildFilesDiagnostics(files, spec.Diagnostics)

	data := &oracle.Data{
		Version:      1,
		Dependencies: deps,
		Files:        files,
	}
	o, err := oracle.LoadFromData(data)
	if err != nil {
		t.Fatalf("LoadFromData: %v", err)
	}
	return o
}

// LookupDiagnostics on the real oracle reads from raw.Files. The raw
// pointer survives even though Declarations and Expressions are zeroed
// after indexing, so per-file diagnostics keep working — but the
// signature returns []Diagnostic (value), and the raw stores
// []*Diagnostic. Real impl converts; the test layer relies on
// that conversion existing. (No code here — note for reviewers.)

func splitOwnerMember(key string) (owner, member string, ok bool) {
	idx := strings.LastIndex(key, ".")
	if idx < 0 {
		return "", "", false
	}
	return key[:idx], key[idx+1:], true
}

func appendMember(deps map[string]*oracle.Class, ownerFQN string, m *oracle.Member) {
	cls, exists := deps[ownerFQN]
	if !exists {
		cls = &oracle.Class{FQN: ownerFQN, Kind: "class"}
		deps[ownerFQN] = cls
	}
	cls.Members = append(cls.Members, m)
}

func packageOf(fqn string) string {
	idx := strings.LastIndex(fqn, ".")
	if idx < 0 {
		return ""
	}
	return fqn[:idx]
}

func joinPackage(pkg, simple string) string {
	if pkg == "" {
		return simple
	}
	return pkg + "." + simple
}
