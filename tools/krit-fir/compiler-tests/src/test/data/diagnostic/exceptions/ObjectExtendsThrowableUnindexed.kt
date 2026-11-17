// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 15, 22, 30
// Objects declared in a companion object body or an enum class body, which
// Go's declaration index never descends into. Go reads each object's own
// superclass from the source, so the container does not matter, in Go or
// here.
package test

// --- Positives: Throwable objects ---

class CompanionHost {
    companion object {
        object <!ObjectExtendsThrowable!>Busy<!> : IllegalStateException()

        object <!ObjectExtendsThrowable!>Closed<!> : Exception("closed")
    }
}

enum class EnumHost {
    A;

    object <!ObjectExtendsThrowable!>EInner<!> : Exception()
}

class NestedHost {
    companion object {
        // Not reported: Outer2 extends nothing; only the nested object is a
        // Throwable.
        object Outer2 {
            object <!ObjectExtendsThrowable!>Inner2<!> : Exception()
        }
    }
}

// --- Negatives: objects that only mention an exception type ---

class C1 {
    companion object {
        // A member type annotation `: Exception?`; InCompanion extends nothing.
        object InCompanion {
            val cause: Exception? = null
        }
    }
}

interface ErrorHandler

class HandlerHost {
    companion object {
        // `: ErrorHandler` starts with `: Error`; HandlerHolder is an
        // ErrorHandler.
        object HandlerHolder : ErrorHandler
    }
}

enum class E {
    A;

    // A member type annotation `: Error?`; EInner2 extends nothing.
    object EInner2 {
        val e: Error? = null
    }
}
