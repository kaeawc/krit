// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 6, 10, 12, 16, 21, 23, 25, 27, 30, 36, 47
// Positives: a private class, interface, enum or annotation that nothing in its file refers to.
package test.unusedprivateclass

private class <!UnusedPrivateClass!>Helper<!> {
    fun doWork() = 1
}

private data class <!UnusedPrivateClass!>Pair2<!>(val a: Int, val b: Int)

private interface <!UnusedPrivateClass!>Contract<!> {
    fun run()
}

private enum class <!UnusedPrivateClass!>Mode<!> {
    A,
    B,
}

private annotation class <!UnusedPrivateClass!>Marker<!>

private sealed class <!UnusedPrivateClass!>Sealed<!>

private abstract class <!UnusedPrivateClass!>Base<!>

private open class <!UnusedPrivateClass!>Opened<!>

class Holder {
    private class <!UnusedPrivateClass!>NestedUnused<!>

    fun go() = 1
}

// A class that only mentions itself is unused, like Go.
private class <!UnusedPrivateClass!>SelfReferencing<!> {
    fun copy() = SelfReferencing()

    companion object {
        val DEFAULT = SelfReferencing()
    }
}

// A class whose only mention is from a class that is itself unused is still mentioned, so it is kept.
private class Mentioned

private class <!UnusedPrivateClass!>Mentioner<!> {
    val inner = Mentioned()
}
