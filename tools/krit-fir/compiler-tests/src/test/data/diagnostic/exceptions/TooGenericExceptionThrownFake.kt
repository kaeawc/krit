// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8, 12, 16
// Fakes throw generic exceptions on purpose to simulate failures, so neither
// Go nor FIR reports a throw inside a Fake*/Test* type. A plain class still is.
package test

class FakeRepository {
    fun load(): Nothing = throw Exception("simulated failure")
}

object TestFailures {
    fun fail(): Nothing = throw RuntimeException("simulated failure")
}

class Repository {
    fun load(): Nothing = throw <!TooGenericExceptionThrown!>Exception("real failure")<!>
}
