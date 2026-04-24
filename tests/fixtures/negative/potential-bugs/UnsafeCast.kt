package potentialbugs

// Safe cast operator — never triggers UnsafeCast.
class UnsafeCastSafeCastOp {
    fun example(x: Any): String? {
        return x as? String
    }
}

// is-check guard in if-body.
class UnsafeCastIfIsGuard {
    open class Base
    class Derived : Base()

    fun example(x: Base): Derived {
        if (x is Derived) {
            return x as Derived
        }
        throw IllegalArgumentException("expected Derived, got ${x::class.simpleName}")
    }
}

// Negative is-check with early return guard (single-expression body).
class UnsafeCastNegIsEarlyReturn {
    open class Base
    class Derived : Base()

    fun example(x: Base): Derived {
        if (x !is Derived) throw IllegalArgumentException("expected Derived, got ${x::class.simpleName}")
        return x as Derived
    }
}

// Negative is-check with early throw in braced body.
class UnsafeCastNegIsEarlyThrow {
    open class Base
    class Derived : Base()

    fun example(x: Base): Derived {
        if (x !is Derived) {
            throw IllegalStateException("expected Derived, got ${x::class.simpleName}")
        }
        return x as Derived
    }
}

// when-entry is-check guard.
class UnsafeCastWhenIsGuard {
    open class Base
    class Derived : Base()

    fun example(x: Base): String {
        return when (x) {
            is Derived -> (x as Derived).toString()
            else -> "other"
        }
    }
}

// Multiline condition — AST-based check handles whitespace variation.
class UnsafeCastMultilineCondition {
    open class Base
    class Derived : Base()

    fun example(x: Base): Derived {
        if (x
            is Derived
        ) {
            return x as Derived
        }
        throw IllegalArgumentException("expected Derived, got ${x::class.simpleName}")
    }
}

// Conjunction condition with is-check.
class UnsafeCastConjunctionIsGuard {
    open class Base
    class Derived : Base()

    fun example(x: Base, flag: Boolean): Derived {
        if (x is Derived && flag) {
            return x as Derived
        }
        throw IllegalArgumentException("expected Derived with flag, got ${x::class.simpleName}")
    }
}

// equals() method — `other as ThisClass` is the IntelliJ-generated pattern.
class UnsafeCastEqualsMethod {
    override fun equals(other: Any?): Boolean {
        if (other !is UnsafeCastEqualsMethod) return false
        val o = other as UnsafeCastEqualsMethod
        return o === this
    }

    override fun hashCode(): Int = System.identityHashCode(this)
}
