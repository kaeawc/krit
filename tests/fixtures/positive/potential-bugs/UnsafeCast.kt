package potentialbugs

// Bare unsafe cast — no guard.
class UnsafeCast {
    fun example(x: Any): String {
        return x as String
    }
}

// Comment containing is-check text must NOT suppress the rule.
class UnsafeCastCommentFalseGuard {
    open class Base
    class Target : Base()

    fun example(x: Base): Target {
        // x is Target — this comment must not suppress the finding
        return x as Target
    }
}

// String literal containing is-check text must NOT suppress the rule.
class UnsafeCastStringLiteralFalseGuard {
    open class Base
    class Target : Base()

    fun example(x: Base): Target {
        check("x is Target".isNotEmpty()) { "guard text" }
        return x as Target
    }
}

// Nested if with no else — body does NOT always exit, guard is not proven.
class UnsafeCastNestedNonExitingGuard {
    open class Base
    class Target : Base()

    fun example(x: Base, cond: Boolean): Target {
        if (x !is Target) {
            if (cond) throw IllegalStateException("not a Target")
            // falls through when cond is false
        }
        return x as Target
    }
}
