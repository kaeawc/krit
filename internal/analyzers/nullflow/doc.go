// Package nullflow provides flow-sensitive nullability analysis primitives
// over a parsed scanner.File. It owns the proofs that a given use site is
// non-null via guards, early-return, short-circuit, same-block assignment,
// post-filter smart casts, contains-key checks, and explicit-this reference
// matching.
//
// Rules consume nullflow.* APIs instead of open-coding the same flat-AST
// walks. The package depends only on the scanner package; it does not import
// the rules package and must not.
package nullflow
