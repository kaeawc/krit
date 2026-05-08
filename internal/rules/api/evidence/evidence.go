// Package evidence is a thin facade over scanner.File + the wired type
// resolvers, exposing only the structured questions a rule should ask.
//
// Rule bodies that work through Evidence get receiver-typing, callee-name,
// and argument navigation through one API. Direct *scanner.File access
// (and the substring-on-source-text patterns it invites) stays available
// in the rules package today, but new rules should prefer this layer so
// the cheap-receiver-typing path is shared and memoized.
package evidence

import (
	"github.com/kaeawc/krit/internal/javafacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// OwnerSource records which backend proved a call's owner type.
// OwnerUnknown means no backend could prove it — rules requiring
// receiver proof must bail out on this value.
type OwnerSource uint8

const (
	OwnerUnknown OwnerSource = iota
	// OwnerImportEvidence: receiver was a type-name receiver (e.g.
	// `Foo.bar()`) and the file's import table named the FQN.
	OwnerImportEvidence
	// OwnerResolver: the in-process source resolver returned an FQN
	// (Kotlin scopes, parameter types, property declarations).
	OwnerResolver
	// OwnerJavaSource: the Java AST + JavaFacts produced an FQN
	// from a parameter, field, or local declaration in scope.
	OwnerJavaSource
)

// Evidence wraps a Context's read-only inputs with cached per-file lookups.
// One Evidence per rule invocation is fine — the underlying caches live on
// the file, not on the Evidence value.
type Evidence struct {
	file      *scanner.File
	resolver  typeinfer.TypeResolver
	javaFacts *javafacts.JavaFileFacts
	javaIndex *javafacts.SourceIndex

	// ownerCache memoizes ResolveOwner answers within this Evidence's lifetime.
	ownerCache map[uint32]ownerEntry
}

type ownerEntry struct {
	fqn    string
	source OwnerSource
}

// From wraps the rule's Context. Returns nil only when ctx itself is nil.
func From(ctx *api.Context) *Evidence {
	if ctx == nil {
		return nil
	}
	return &Evidence{
		file:      ctx.File,
		resolver:  ctx.Resolver,
		javaFacts: ctx.JavaFacts,
		javaIndex: ctx.JavaSourceIndex,
	}
}

// File returns the underlying file. Provided for emit-position math
// (line/col lookups) — rules should not use it to read source text.
func (e *Evidence) File() *scanner.File { return e.file }
