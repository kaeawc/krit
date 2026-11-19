// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12
// Go reports a qualified call on the callee name, so a call split across lines
// keeps the finding on `.synchronized`.
package test

class Holder {
    private var lock = Any()

    fun qualified() {
        kotlin
            .<!SynchronizedOnNonFinal!>synchronized<!>(lock) { }
    }

    fun reassign() {
        lock = Any()
    }
}
