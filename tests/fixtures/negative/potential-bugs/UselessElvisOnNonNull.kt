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

    // Member access on a receiver whose type is not declared in this file.
    // Source inference cannot see the member's declared nullability and must
    // not default it to non-null: the member may well be nullable, so the
    // elvis fallback is meaningful.
    fun memberAccessOnUnknownReceiver(decl: ExternalDecl) {
        val y = decl.source ?: return
        use(y)
    }

    // Member access to a name that does not exist on a known in-file class.
    // The receiver type resolves, but the member does not — its nullability
    // is unproven, so the rule must not fire.
    fun memberAccessUnknownMember(h: TestHarness) {
        val y = h.missing ?: "fallback"
        use(y)
    }

    // Qualified call whose target cannot be resolved in-file or in the stdlib.
    // The return type's nullability is unproven; the elvis must be preserved.
    fun qualifiedCallUnknownTarget(factory: ExternalFactory) {
        val y = factory.create() ?: return
        use(y)
    }

    fun use(x: Any?) {}
}

class TestHarness {
    val group: String? = null
}
