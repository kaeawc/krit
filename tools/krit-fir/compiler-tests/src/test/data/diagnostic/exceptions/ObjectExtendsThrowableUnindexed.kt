// RENDER_DIAGNOSTICS_FULL_TEXT
// Objects declared in a companion object body or an enum class body. Go's
// declaration index never descends into either container, so the resolver
// has no hierarchy for these objects and Go falls back to searching the whole
// object's text for `: Exception`, `: IllegalStateException(`, and the other
// names in its fallback list.
package test

// --- Positives: Throwable objects Go reports through that text fallback ---

class CompanionHost {
    companion object {
        <!ObjectExtendsThrowable!>object<!> Busy : IllegalStateException()

        <!ObjectExtendsThrowable!>object<!> Closed : Exception("closed")
    }
}

enum class EnumHost {
    A;

    <!ObjectExtendsThrowable!>object<!> EInner : Exception()
}

class NestedHost {
    companion object {
        // Go reports this because the nested object's text below contains
        // `: Exception()`; FIR is correct to drop it because Outer2 extends
        // nothing and is not a Throwable.
        object Outer2 {
            <!ObjectExtendsThrowable!>object<!> Inner2 : Exception()
        }
    }
}

// --- Negatives: Go reports these only because of the text fallback ---

class C1 {
    companion object {
        // Go reports this because the member type annotation `: Exception?`
        // matches its text search; FIR is correct to drop it because
        // InCompanion extends nothing and is not a Throwable.
        object InCompanion {
            val cause: Exception? = null
        }
    }
}

interface ErrorHandler

class HandlerHost {
    companion object {
        // Go reports this because `: ErrorHandler` starts with `: Error`;
        // FIR is correct to drop it because HandlerHolder is an ErrorHandler,
        // not a Throwable.
        object HandlerHolder : ErrorHandler
    }
}

enum class E {
    A;

    // Go reports this because the member type annotation `: Error?` matches
    // its text search; FIR is correct to drop it because EInner2 extends
    // nothing and is not a Throwable.
    object EInner2 {
        val e: Error? = null
    }
}
