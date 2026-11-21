// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 7
package referentiallookalike

// Known Go false positive: another file's same-package String outranks the default import.
// FIR sees the shared stub; the file-local Go golden resolver does not index that stub.
fun same(a: String, candidate: Any) = a === candidate
