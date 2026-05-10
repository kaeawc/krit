// Package binsymbols defines the seam through which Krit consumes
// class metadata that lives outside the source workspace — JARs, AARs,
// stdlib stubs, or KAA-backed oracle responses.
//
// The package is intentionally narrow: it owns the Class shape that
// callers (typeinfer, rules) consult when source-side lookups miss,
// and a Reader interface that producers implement. No producer is
// included here; concrete readers live in their own packages so this
// one stays free of JVM, archive/zip, or oracle dependencies.
//
// Existing fallbacks (KnownClassHierarchy, KnownInterfaces in
// typeinfer/stdlib.go) are unaffected. A typical wiring layers them:
//
//	source workspace -> binsymbols.Reader -> hardcoded tables
//
// so the reader fills the gap between checked-in source and the small
// hardcoded stdlib stubs.
package binsymbols

import "sort"

// Class is the shape returned by readers. It mirrors the subset of
// typeinfer.ClassInfo that the rule layer reads and is intentionally
// duplicated to keep this package free of typeinfer imports.
type Class struct {
	Name       string   // Simple class name
	FQN        string   // Fully qualified name
	Kind       string   // "class", "interface", "object", "enum", "sealed class", "sealed interface"
	Supertypes []string // FQNs of direct supertypes
	IsAbstract bool
	IsSealed   bool
	Members    []Member
}

// Member is a class member as exposed by readers. Mirrors
// typeinfer.MemberInfo's exposed fields.
type Member struct {
	Name       string
	Kind       string // "function", "property"
	ReturnType string // FQN; empty when unknown
	Visibility string // "public", "private", "internal", "protected"
	IsAbstract bool
	IsOverride bool
	// Params records the parameter types as FQNs in declaration order.
	// Names are not preserved — readers that derive members from JVM
	// descriptors only have type info.
	ParamTypes []string
}

// Reader is the producer interface. A reader returns nil from
// LookupClass when it doesn't know the FQN; nil is normal and indicates
// the caller should fall through to the next reader (or hardcoded
// tables, or a "type unknown" result).
//
// Implementations must be safe for concurrent use — the resolver and
// dispatcher fan out in parallel.
type Reader interface {
	LookupClass(fqn string) *Class
}

// Empty is a Reader that always returns nil. Use as a default when no
// classpath-aware reader is configured.
var Empty Reader = emptyReader{}

type emptyReader struct{}

func (emptyReader) LookupClass(string) *Class { return nil }

// Multi composes a chain of readers. LookupClass tries each in order
// and returns the first non-nil result. Nil entries are skipped.
type Multi struct {
	Readers []Reader
}

// LookupClass implements Reader.
func (m Multi) LookupClass(fqn string) *Class {
	for _, r := range m.Readers {
		if r == nil {
			continue
		}
		if c := r.LookupClass(fqn); c != nil {
			return c
		}
	}
	return nil
}

// Static is a deterministic in-memory Reader. Useful for tests and for
// the small per-project allowlists that callers stage before a real
// reader plugs in. Concurrent reads are safe; do not mutate the map
// after the Static value escapes.
type Static struct {
	Classes map[string]*Class
}

// LookupClass implements Reader.
func (s Static) LookupClass(fqn string) *Class {
	if s.Classes == nil {
		return nil
	}
	if c, ok := s.Classes[fqn]; ok {
		return c
	}
	return nil
}

// FQNs returns the sorted set of FQNs known to the static reader. A
// debug helper that keeps the cmdline outputs deterministic.
func (s Static) FQNs() []string {
	out := make([]string, 0, len(s.Classes))
	for fqn := range s.Classes {
		out = append(out, fqn)
	}
	sort.Strings(out)
	return out
}
