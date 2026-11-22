// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

open class ReflectiveBase
class ReflectiveDerived : ReflectiveBase()

// UnsafeCast.kt lines 19, 29, 41, 71, 84: KClass.simpleName is String?, which Go does not resolve.
// An anonymous object : ReflectiveBase() reaches every throw below and has a null simpleName.
// Resolution therefore proves these FIR-only findings even though the receiver itself is non-null.
fun reflectiveIfGuard(value: ReflectiveBase): ReflectiveDerived {
    if (value is ReflectiveDerived) return value
    throw IllegalArgumentException("expected Derived, got <!NullableToStringCall!>${value::class.simpleName}<!>")
}

fun reflectiveEarlyReturn(value: ReflectiveBase): ReflectiveDerived {
    if (value !is ReflectiveDerived) throw IllegalArgumentException("expected Derived, got <!NullableToStringCall!>${value::class.simpleName}<!>")
    return value
}

fun reflectiveEarlyThrow(value: ReflectiveBase): ReflectiveDerived {
    if (value !is ReflectiveDerived) {
        throw IllegalStateException("expected Derived, got <!NullableToStringCall!>${value::class.simpleName}<!>")
    }
    return value
}

fun reflectiveMultilineGuard(value: ReflectiveBase): ReflectiveDerived {
    if (value
        is ReflectiveDerived
    ) return value
    throw IllegalArgumentException("expected Derived, got <!NullableToStringCall!>${value::class.simpleName}<!>")
}

fun reflectiveConjunctionGuard(value: ReflectiveBase, flag: Boolean): ReflectiveDerived {
    if (value is ReflectiveDerived && flag) return value
    throw IllegalArgumentException("expected Derived with flag, got <!NullableToStringCall!>${value::class.simpleName}<!>")
}
