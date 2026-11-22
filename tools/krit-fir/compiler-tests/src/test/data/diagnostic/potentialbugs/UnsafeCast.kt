// go-lines: 7, 8, 9, 10, 11, 12, 15, 26, 28, 33, 37, 38, 46, 50
package unsafecastbasic

class CastLeft
class CastRight

fun impossibleLiteral() = <!UnsafeCast!>1 as String<!>
fun impossibleSafeLiteral() = <!UnsafeCast!>1 as? String<!>
fun impossibleClasses(value: CastLeft) = <!UnsafeCast!>value as CastRight<!>
fun impossibleSafeClasses(value: CastLeft) = <!UnsafeCast!>value as? CastRight<!>
fun impossibleNullableSource(value: CastLeft?) = <!UnsafeCast!>value as CastRight<!>
fun impossibleSafeNullableSource(value: CastLeft?) = <!UnsafeCast!>value as? CastRight<!>
// FIR-only: Go's subtype shortcut accepts Nothing?; null cannot inhabit the resolved non-null String target.
fun impossibleNull() = <!UnsafeCast!>null as String<!>
fun nestedImpossible() = (<!UnsafeCast!>1 as String<!>) as CharSequence
fun compatibleFinal(value: String) = value as String
fun compatibleBase(value: String) = value as CharSequence
fun anyDowncast(value: Any) = value as String
fun anySafeDowncast(value: Any) = value as? String
fun nullableTarget(value: Int) = value as String?
fun safeNullableTarget(value: Int) = value as? String?
fun nullableNull() = null as String?
fun safeNull() = null as? String

class CastContainer {
    fun member(value: CastLeft) = <!UnsafeCast!>value as CastRight<!>
    companion object {
        fun companion(value: CastLeft) = <!UnsafeCast!>value as CastRight<!>
    }
}

fun anonymousScope(value: CastLeft) = object {
    fun impossible() = <!UnsafeCast!>value as CastRight<!>
}

fun localScope(value: CastLeft) {
    fun local() = <!UnsafeCast!>value as CastRight<!>
    val action = { <!UnsafeCast!>value as CastRight<!> }
}

fun previewAnonymousFunction() {
    val render = fun(): String = 1 as String
}

fun productionAnonymousFunction() {
    val render = fun(): String = <!UnsafeCast!>1 as String<!>
}

@Deprecated("legacy")
fun deprecatedDoesNotHideImpossibleCast() = <!UnsafeCast!>1 as String<!>

// FIR emits the raw diagnostic; Go's postpass applies the UNCHECKED_CAST suppression.
@Suppress("UNCHECKED_CAST")
fun compilerFindingIsSuppressedInProduction() = <!UnsafeCast!>1 as String<!>
