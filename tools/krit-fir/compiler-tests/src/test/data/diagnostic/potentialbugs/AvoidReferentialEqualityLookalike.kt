// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9
package lookalike

class String
fun sameNamedType(a: String, b: String) = a === b
// FIR recognizes the qualified builtin; Go does not recover unresolved identifiers.
fun realStrings(a: kotlin.String, b: kotlin.String) = <!AvoidReferentialEquality!>a !== b<!>
fun literalEvenWithLookalike(candidate: Any) = <!AvoidReferentialEquality!>candidate === "a"<!>

// Go stays silent when its resolver misses the qualified builtin, including captures.
fun capturedStrings(a: kotlin.String, b: kotlin.String) = run { <!AvoidReferentialEquality!>a === b<!> }
