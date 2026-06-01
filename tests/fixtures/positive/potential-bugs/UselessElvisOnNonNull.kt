package fixtures.positive.potentialbugs

class UselessElvisOnNonNull {
    fun nonNullLocal() {
        val x: String = "hello"
        val y = x ?: "fallback"
    }

    fun nonNullLiteral() {
        val y = "always" ?: "dead"
    }

    fun nonNullInt() {
        val n: Int = 42
        val m = n ?: 0
    }

    // Member access on a local val whose class is declared in this file and
    // whose member is non-null — the target is proven, so the fallback is dead.
    fun nonNullMemberOnLocal() {
        val h = Holder()
        val v = h.label ?: "dead"
    }

    // Member access via `this` on a non-null property — also proven non-null.
    val title: String = "t"
    fun nonNullMemberOnThis() {
        val v = this.title ?: "dead"
    }
}

class Holder {
    val label: String = ""
}
