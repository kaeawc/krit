package fixtures.negative.potentialbugs

class UselessElvisOnNonNull {
    // Nullable parameter — elvis is meaningful.
    fun nullableParam(s: String?) {
        val y = s ?: "fallback"
    }

    // Mutable var — Kotlin cannot smart-cast; the source resolver should not
    // claim non-null even when initialized with a non-null literal.
    var name: String = "hello"
    fun mutableVar() {
        val y = name ?: "fallback"
    }

    // Safe call chain — left side is nullable.
    fun safeCallChain(s: String?) {
        val y = s?.length ?: 0
    }

    // Dotted member access on unresolved receiver — bare-name resolver
    // cannot prove non-null.
    fun dottedAccess(harness: TestHarness?) {
        val y = harness?.group ?: "fallback"
    }

    // Function call that may return null by convention.
    fun findOrFallback(items: List<String>) {
        val y = items.find { it.isEmpty() } ?: "fallback"
    }

    // Call-expression whose callee is a safe-call chain. The call's
    // declared return type is non-nullable Int, but the chain short-
    // circuits to null when `s` is null, so the elvis fallback is real.
    fun safeCallTerminatingInCall(s: String?) {
        val y = s?.length?.toInt() ?: 0
    }

    // Navigation chain that ends in `.bar` whose receiver is a safe-call
    // call_expression. The outer step is `.`, but the receiver propagates
    // null upstream — the elvis must be preserved.
    fun navigationAfterSafeCall(harness: TestHarness?) {
        val y = harness?.group?.length.toString() ?: "fallback"
    }
}

class TestHarness {
    val group: String? = null
}
