// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 13, 17
// Divergence (exemption): fakes throw generic exceptions on purpose to simulate
// failures, so FIR skips a throw inside a Fake*/Stub*/Test* type. Go's Kotlin
// rule, like detekt, reports them. A plain class is reported by both.
package test

class FakeRepository {
    fun load(): Nothing = throw Exception("simulated failure")
}

object TestFailures {
    fun fail(): Nothing = throw RuntimeException("simulated failure")
}

class Repository {
    fun load(): Nothing = <!TooGenericExceptionThrown!>throw Exception("real failure")<!>
}
