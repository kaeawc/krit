# FQNSymbolIndex

**Cluster:** [lsp](README.md) · **Status:** open ·
**Severity:** n/a (infra) · **Default:** n/a · **Est.:** 2–3 days

## What it does

Expose the oracle's declaration and expression data as a
reverse-lookup index keyed by fully qualified name (FQN). Gives
higher-level consumers three primitives:

- `FindDeclarationByFQN(fqn string) (*DeclLocation, bool)`
- `FindReferencesByFQN(fqn string) []ReferenceLocation`
- `TypeAtExpression(exprId string) (*TypeInfo, bool)`

These are the three operations every LSP navigation handler needs.
Every rule that currently asks the oracle for "what type is this
expression" can also migrate to `TypeAtExpression` once the shape
stabilizes.

## Current cost

The oracle JSON assembled by
[`internal/oracle/assemble.go`](../../../internal/oracle/) already
contains:

- `declarations: map[string]DeclInfo` — keyed by FQN, one per class
  / object / function / property / typealias. Carries `file`, `line`,
  `column`, `kind`, `signature`.
- `expressions: map[string]ExprInfo` — keyed by expression id,
  carries resolved-call FQN, type, and source range.

But consumers today only walk these maps linearly. To answer "find
all references to `kotlinx.coroutines.CoroutineScope`" today, a
caller has to iterate every file's `expressions` map and filter by
`resolvedCall.fqn == X`. That's O(total expressions) per query —
fine for one-shot rule checks, too slow for interactive LSP where
a user hovers 20 times per minute.

## Proposed design

### Types

```go
// internal/oracle/index.go

type DeclLocation struct {
    FQN       string
    Kind      DeclKind // class, object, function, property, typealias
    File      string   // absolute path
    Line      int      // 1-based
    Column    int      // 1-based
    Signature string   // rendered, e.g. "fun foo(x: Int): String"
}

type ReferenceLocation struct {
    FQN    string // the symbol being referenced
    File   string
    Line   int
    Column int
    // true when this reference *is* the declaration itself (used to
    // filter it out on Find References when IncludeDeclaration=false)
    IsDeclaration bool
}

type TypeInfo struct {
    FQN   string // canonical type FQN
    Nullable bool
    Arguments []TypeInfo // for generics; empty for monotypes
}

type Index struct {
    decls map[string]*DeclLocation       // FQN → decl
    refs  map[string][]ReferenceLocation // FQN → all uses (including decl)
    exprs map[string]*TypeInfo            // expression id → type
}
```

### Build

```go
// internal/oracle/index.go

// BuildIndex constructs the reverse lookup from an assembled Oracle.
// Runs once per full oracle assembly; reused across queries until the
// oracle is invalidated.
func BuildIndex(o *Oracle) *Index
```

Implementation: one linear pass over `o.Declarations`, one pass over
every file's `Expressions`. O(total declarations + total expressions).
On Signal-Android this is ~2,400 files × ~few-hundred declarations +
several-thousand expressions ≈ <100ms total build on warm oracle.
Memory cost: roughly equal to the oracle JSON itself, since this is a
reshaping of existing data.

### Query

All three query methods are O(1) on the `map[string]` lookup plus the
size of the result slice.

```go
func (idx *Index) FindDeclarationByFQN(fqn string) (*DeclLocation, bool)
func (idx *Index) FindReferencesByFQN(fqn string) []ReferenceLocation
func (idx *Index) TypeAtExpression(exprId string) (*TypeInfo, bool)
```

### Lifecycle

- `oracle.Oracle` gains an optional `*Index` field, lazily initialized
  the first time any index method is called.
- Oracle invalidation (cache bust, full rebuild) clears the index.
- Partial invalidation from a single-file edit (see
  [`didchange-oracle-refresh.md`](didchange-oracle-refresh.md)) rebuilds
  only the entries touched by that file.

## Files to touch

- `internal/oracle/index.go` — new file, ~250 lines
- `internal/oracle/index_test.go` — unit tests using small hand-built
  oracle fixtures
- `internal/oracle/oracle.go` — add `Index() *Index` accessor with
  lazy init + sync.Once

## Testing

- Fixture 1: 3 Kotlin files, one top-level function in each, one of
  them calls the other two. Assertions on `FindDeclarationByFQN`,
  `FindReferencesByFQN` counts and positions.
- Fixture 2: class hierarchy with overridden methods. Verify that
  `CoroutineScope.launch` references don't get lumped in with
  `MyScope.launch` references (FQN discrimination).
- Fixture 3: expression types for literals, calls, property accesses.
  Verify nullability and generic arg recovery.
- Fixture 4: empty file, file with only imports, file with a single
  typealias. Build should succeed with empty result sets.

## Risks

- **Index memory**. The index roughly doubles the oracle's in-memory
  footprint. Acceptable on Signal-scale; may need eviction on
  kotlin/kotlin scale (18k files).
- **Build latency on cold path**. A linear pass over the oracle is
  cheap *if* the oracle is in memory. On a cold cache miss that
  requires re-assembling the oracle first, the 1–2s oracle build
  dominates.
- **FQN canonicalization**. `kotlinx.coroutines.CoroutineScope` and
  `CoroutineScope` (after import) are the same symbol. The oracle
  stores canonical FQNs, but the query surface must also accept
  unqualified names when given scope context. Scope resolution is
  deferred to milestone 2.

## Blocking

- None. Uses only existing oracle output.

## Blocked by

- None — this is the cluster's foundation milestone.

## Links

- Parent cluster: [`lsp/README.md`](README.md)
- Consumes: [`internal/oracle/`](../../../internal/oracle/) assembled
  oracle output
- Consumer: [`navigation-handlers-rewrite.md`](navigation-handlers-rewrite.md)
