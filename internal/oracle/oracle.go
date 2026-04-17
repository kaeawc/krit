package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/kaeawc/krit/internal/typeinfer"
)

// Lookup is the interface for querying oracle type information.
// Implementations: *Oracle (real), *FakeOracle (test double).
type Lookup interface {
	LookupClass(name string) *typeinfer.ClassInfo
	LookupSealedVariants(name string) []string
	LookupEnumEntries(name string) []string
	IsSubtype(a, b string) bool
	Dependencies() map[string]*OracleClass
	LookupFunction(key string) *typeinfer.ResolvedType
	LookupExpression(filePath string, line, col int) *typeinfer.ResolvedType
	LookupAnnotations(key string) []string
	LookupCallTarget(filePath string, line, col int) string
	LookupDiagnostics(filePath string) []OracleDiagnostic
}

// Oracle holds pre-computed type information from the Kotlin compiler.
type Oracle struct {
	raw            *OracleData
	classByFQN     map[string]*typeinfer.ClassInfo
	classBySimple  map[string]*typeinfer.ClassInfo
	sealedVariants map[string][]string
	enumEntries    map[string][]string
	functions      map[string]*typeinfer.ResolvedType
	supertypeMap   map[string][]string                           // FQN → all ancestor FQNs (transitive)
	subtypeSet     map[string]map[string]bool                    // FQN → set of ancestor FQNs for O(1) IsSubtype
	expressions    map[string]map[uint64]*typeinfer.ResolvedType // file path → (packed line:col → type)
	annotations    map[string][]string                           // "ClassName.memberName" → annotation FQNs
	callTargets    map[string]map[uint64]string                  // file path → (packed line:col → call target FQN)

	// Hit/miss counters, updated by LookupClass/LookupExpression/LookupFunction.
	exprHits    atomic.Int64
	exprMisses  atomic.Int64
	classHits   atomic.Int64
	classMisses atomic.Int64
	funcHits    atomic.Int64
	funcMisses  atomic.Int64
}

// OracleStats is a snapshot of Oracle lookup hit/miss counters.
type OracleStats struct {
	ExprHits, ExprMisses   int64
	ClassHits, ClassMisses int64
	FuncHits, FuncMisses   int64
}

// Stats returns a snapshot of hit/miss counters for Oracle lookups.
// Not part of the Lookup interface (no fake wiring required).
func (o *Oracle) Stats() OracleStats {
	return OracleStats{
		ExprHits:    o.exprHits.Load(),
		ExprMisses:  o.exprMisses.Load(),
		ClassHits:   o.classHits.Load(),
		ClassMisses: o.classMisses.Load(),
		FuncHits:    o.funcHits.Load(),
		FuncMisses:  o.funcMisses.Load(),
	}
}

// packLineCol packs a 1-based source position into a single uint64 so that
// expression and call-target lookups avoid allocating a "line:col" string on
// every call.
func packLineCol(line, col int) uint64 {
	return uint64(line)<<32 | uint64(col)
}

// parseLineCol parses a "line:col" string (from the oracle JSON) into a
// packed uint64 key. Returns false if the string is malformed or contains
// negative numbers.
func parseLineCol(s string) (uint64, bool) {
	colon := strings.IndexByte(s, ':')
	if colon <= 0 || colon == len(s)-1 {
		return 0, false
	}
	line, err := strconv.Atoi(s[:colon])
	if err != nil || line < 0 {
		return 0, false
	}
	col, err := strconv.Atoi(s[colon+1:])
	if err != nil || col < 0 {
		return 0, false
	}
	return uint64(line)<<32 | uint64(col), true
}

// Load reads a JSON oracle file and builds lookup indexes.
func Load(path string) (*Oracle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read oracle: %w", err)
	}

	var raw OracleData
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse oracle JSON: %w", err)
	}

	if raw.Version < 1 {
		return nil, fmt.Errorf("unsupported oracle version: %d", raw.Version)
	}

	o := &Oracle{
		raw:            &raw,
		classByFQN:     make(map[string]*typeinfer.ClassInfo),
		classBySimple:  make(map[string]*typeinfer.ClassInfo),
		sealedVariants: make(map[string][]string),
		enumEntries:    make(map[string][]string),
		functions:      make(map[string]*typeinfer.ResolvedType),
		supertypeMap:   make(map[string][]string),
		subtypeSet:     make(map[string]map[string]bool),
		expressions:    make(map[string]map[uint64]*typeinfer.ResolvedType),
		annotations:    make(map[string][]string),
		callTargets:    make(map[string]map[uint64]string),
	}

	// Index dependency types
	for fqn, cls := range raw.Dependencies {
		o.indexClass(fqn, cls)
	}

	// Index source file declarations and expressions
	for path, file := range raw.Files {
		for _, cls := range file.Declarations {
			if cls.FQN != "" {
				o.indexClass(cls.FQN, cls)
			}
		}
		if len(file.Expressions) > 0 {
			exprMap := make(map[uint64]*typeinfer.ResolvedType, len(file.Expressions))
			var ctMap map[uint64]string
			for pos, et := range file.Expressions {
				key, ok := parseLineCol(pos)
				if !ok {
					continue
				}
				exprMap[key] = makeResolvedType(et.Type, et.Nullable)
				if et.CallTarget != "" {
					if ctMap == nil {
						ctMap = make(map[uint64]string)
					}
					ctMap[key] = et.CallTarget
				}
			}
			o.expressions[path] = exprMap
			if ctMap != nil {
				o.callTargets[path] = ctMap
			}
		}
	}

	// Build transitive supertype map
	o.buildSupertypeMap()

	// Free raw per-file data we've already indexed to reduce memory footprint.
	// NOTE: o.raw.Dependencies is intentionally retained because Oracle.Dependencies()
	// is called by cmd/krit post-Load for reporting. Diagnostics is also retained
	// because LookupDiagnostics reads it lazily.
	for _, f := range o.raw.Files {
		f.Declarations = nil
		f.Expressions = nil
	}

	return o, nil
}

// LoadFromData builds an Oracle from an already-parsed OracleData struct
// (e.g., from a daemon response) without reading from disk.
func LoadFromData(raw *OracleData) (*Oracle, error) {
	if raw == nil {
		return nil, fmt.Errorf("nil oracle data")
	}
	if raw.Version < 1 {
		return nil, fmt.Errorf("unsupported oracle version: %d", raw.Version)
	}

	o := &Oracle{
		raw:            raw,
		classByFQN:     make(map[string]*typeinfer.ClassInfo),
		classBySimple:  make(map[string]*typeinfer.ClassInfo),
		sealedVariants: make(map[string][]string),
		enumEntries:    make(map[string][]string),
		functions:      make(map[string]*typeinfer.ResolvedType),
		supertypeMap:   make(map[string][]string),
		subtypeSet:     make(map[string]map[string]bool),
		expressions:    make(map[string]map[uint64]*typeinfer.ResolvedType),
		annotations:    make(map[string][]string),
		callTargets:    make(map[string]map[uint64]string),
	}

	for fqn, cls := range raw.Dependencies {
		o.indexClass(fqn, cls)
	}

	for path, file := range raw.Files {
		for _, cls := range file.Declarations {
			if cls.FQN != "" {
				o.indexClass(cls.FQN, cls)
			}
		}
		if len(file.Expressions) > 0 {
			exprMap := make(map[uint64]*typeinfer.ResolvedType, len(file.Expressions))
			var ctMap map[uint64]string
			for pos, et := range file.Expressions {
				key, ok := parseLineCol(pos)
				if !ok {
					continue
				}
				exprMap[key] = makeResolvedType(et.Type, et.Nullable)
				if et.CallTarget != "" {
					if ctMap == nil {
						ctMap = make(map[uint64]string)
					}
					ctMap[key] = et.CallTarget
				}
			}
			o.expressions[path] = exprMap
			if ctMap != nil {
				o.callTargets[path] = ctMap
			}
		}
	}

	o.buildSupertypeMap()

	// Free raw per-file data we've already indexed to reduce memory footprint.
	// See Load() for rationale on retaining Dependencies and Diagnostics.
	for _, f := range o.raw.Files {
		f.Declarations = nil
		f.Expressions = nil
	}

	return o, nil
}

func (o *Oracle) indexClass(fqn string, cls *OracleClass) {
	info := convertClassInfo(fqn, cls)
	o.classByFQN[fqn] = info

	simpleName := fqn
	if idx := strings.LastIndex(fqn, "."); idx >= 0 {
		simpleName = fqn[idx+1:]
	}
	o.classBySimple[simpleName] = info

	// Track sealed variants
	if !cls.IsSealed {
		for _, st := range cls.Supertypes {
			o.sealedVariants[st] = append(o.sealedVariants[st], simpleName)
			if stSimple := simpleNameOf(st); stSimple != st {
				o.sealedVariants[stSimple] = append(o.sealedVariants[stSimple], simpleName)
			}
		}
	}

	// Track enum entries from members
	if cls.Kind == "enum" {
		var entries []string
		for _, m := range cls.Members {
			if m.Kind == "enum_entry" {
				entries = append(entries, m.Name)
			}
		}
		if len(entries) > 0 {
			o.enumEntries[fqn] = entries
			o.enumEntries[simpleName] = entries
		}
	}

	// Index member functions with their return types, and member annotations
	for _, m := range cls.Members {
		if m.Kind == "function" && m.ReturnType != "" {
			key := simpleName + "." + m.Name
			o.functions[key] = makeResolvedType(m.ReturnType, m.Nullable)
			fqnKey := fqn + "." + m.Name
			o.functions[fqnKey] = makeResolvedType(m.ReturnType, m.Nullable)
		}
		if len(m.Annotations) > 0 {
			key := simpleName + "." + m.Name
			o.annotations[key] = m.Annotations
			fqnKey := fqn + "." + m.Name
			o.annotations[fqnKey] = m.Annotations
		}
	}

	// Index class-level annotations
	if len(cls.Annotations) > 0 {
		o.annotations[simpleName] = cls.Annotations
		o.annotations[fqn] = cls.Annotations
	}
}

func (o *Oracle) buildSupertypeMap() {
	cache := make(map[string][]string)
	var visit func(fqn string) []string
	visit = func(fqn string) []string {
		if a, ok := cache[fqn]; ok {
			return a
		}
		cache[fqn] = nil // sentinel to break cycles
		info, ok := o.classByFQN[fqn]
		if !ok {
			return nil
		}
		seen := make(map[string]bool)
		var result []string
		for _, st := range info.Supertypes {
			if seen[st] {
				continue
			}
			seen[st] = true
			result = append(result, st)
			for _, anc := range visit(st) {
				if !seen[anc] {
					seen[anc] = true
					result = append(result, anc)
				}
			}
		}
		cache[fqn] = result
		return result
	}
	for fqn := range o.classByFQN {
		if a := visit(fqn); len(a) > 0 {
			o.supertypeMap[fqn] = a
			set := make(map[string]bool, len(a))
			for _, anc := range a {
				set[anc] = true
			}
			o.subtypeSet[fqn] = set
		}
	}
}

// Dependencies returns the raw dependency map for reporting.
func (o *Oracle) Dependencies() map[string]*OracleClass {
	return o.raw.Dependencies
}

// LookupClass returns ClassInfo for a type by FQN or simple name.
func (o *Oracle) LookupClass(name string) *typeinfer.ClassInfo {
	if info, ok := o.classByFQN[name]; ok {
		o.classHits.Add(1)
		return info
	}
	if info, ok := o.classBySimple[name]; ok {
		o.classHits.Add(1)
		return info
	}
	o.classMisses.Add(1)
	return nil
}

// LookupSealedVariants returns known sealed variants.
func (o *Oracle) LookupSealedVariants(name string) []string {
	return o.sealedVariants[name]
}

// LookupEnumEntries returns known enum entries.
func (o *Oracle) LookupEnumEntries(name string) []string {
	return o.enumEntries[name]
}

// IsSubtype checks if type a is a subtype of type b using the precomputed
// ancestor set. Constant-time after Load.
func (o *Oracle) IsSubtype(a, b string) bool {
	if a == b {
		return true
	}
	if set, ok := o.subtypeSet[a]; ok {
		return set[b]
	}
	return false
}

func convertClassInfo(fqn string, cls *OracleClass) *typeinfer.ClassInfo {
	simpleName := simpleNameOf(fqn)

	var members []typeinfer.MemberInfo
	for _, m := range cls.Members {
		mi := typeinfer.MemberInfo{
			Name:       m.Name,
			Kind:       m.Kind,
			Visibility: m.Visibility,
			IsOverride: m.IsOverride,
			IsAbstract: m.IsAbstract,
		}
		if m.ReturnType != "" {
			mi.Type = makeResolvedType(m.ReturnType, m.Nullable)
		}
		members = append(members, mi)
	}

	return &typeinfer.ClassInfo{
		Name:       simpleName,
		FQN:        fqn,
		Kind:       cls.Kind,
		Supertypes: cls.Supertypes,
		IsSealed:   cls.IsSealed,
		IsData:     cls.IsData,
		IsOpen:     cls.IsOpen,
		IsAbstract: cls.IsAbstract,
		Members:    members,
	}
}

func makeResolvedType(fqn string, nullable bool) *typeinfer.ResolvedType {
	name := simpleNameOf(fqn)
	kind := typeinfer.TypeClass
	if _, ok := typeinfer.PrimitiveTypes[name]; ok {
		kind = typeinfer.TypePrimitive
	}
	if name == "Unit" {
		kind = typeinfer.TypeUnit
	}
	if name == "Nothing" {
		kind = typeinfer.TypeNothing
	}
	if nullable {
		kind = typeinfer.TypeNullable
	}
	return &typeinfer.ResolvedType{
		Name:     name,
		FQN:      fqn,
		Kind:     kind,
		Nullable: nullable,
	}
}

// LookupFunction returns the return type of a function by key (e.g., "ClassName.methodName").
func (o *Oracle) LookupFunction(key string) *typeinfer.ResolvedType {
	if rt, ok := o.functions[key]; ok {
		o.funcHits.Add(1)
		return rt
	}
	o.funcMisses.Add(1)
	return nil
}

// LookupExpression returns the compiler-resolved type for an expression at a
// specific source position (1-based line and column).
func (o *Oracle) LookupExpression(filePath string, line, col int) *typeinfer.ResolvedType {
	fileExprs := o.expressions[filePath]
	if fileExprs == nil {
		o.exprMisses.Add(1)
		return nil
	}
	if rt, ok := fileExprs[packLineCol(line, col)]; ok {
		o.exprHits.Add(1)
		return rt
	}
	o.exprMisses.Add(1)
	return nil
}

// LookupAnnotations returns annotation FQNs for a class or member key
// (e.g., "ClassName" or "ClassName.memberName").
func (o *Oracle) LookupAnnotations(key string) []string {
	return o.annotations[key]
}

// LookupCallTarget returns the FQN of the resolved call target for an
// expression at a specific source position (1-based line and column).
func (o *Oracle) LookupCallTarget(filePath string, line, col int) string {
	fileCTs := o.callTargets[filePath]
	if fileCTs == nil {
		return ""
	}
	return fileCTs[packLineCol(line, col)]
}

// LookupDiagnostics returns compiler diagnostics for a source file.
func (o *Oracle) LookupDiagnostics(filePath string) []OracleDiagnostic {
	file := o.raw.Files[filePath]
	if file == nil || len(file.Diagnostics) == 0 {
		return nil
	}
	result := make([]OracleDiagnostic, len(file.Diagnostics))
	for i, d := range file.Diagnostics {
		result[i] = *d
	}
	return result
}

// Compile-time check that Oracle implements Lookup.
var _ Lookup = (*Oracle)(nil)

func simpleNameOf(fqn string) string {
	if idx := strings.LastIndex(fqn, "."); idx >= 0 {
		return fqn[idx+1:]
	}
	return fqn
}
