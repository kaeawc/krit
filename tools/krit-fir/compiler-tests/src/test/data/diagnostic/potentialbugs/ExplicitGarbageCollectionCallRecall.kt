// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Positives Go misses: these calls resolve to java.lang.System.gc or Runtime.gc without the literal spelling
// `System.gc()` / `Runtime.getRuntime().gc()`, so they do run the collector. Go compares the call text and reports
// none of them.
package test

import java.lang.Runtime as Rt
import java.lang.System as JavaSystem

typealias Jvm = java.lang.Runtime

class AliasedCleaner {
    private val runtime = Rt.getRuntime()

    fun importAlias() {
        <!ExplicitGarbageCollectionCall!>JavaSystem.gc()<!>
        <!ExplicitGarbageCollectionCall!>Rt.getRuntime().gc()<!>
    }

    fun typeAlias() {
        <!ExplicitGarbageCollectionCall!>Jvm.getRuntime().gc()<!>
    }

    fun heldInProperty() {
        <!ExplicitGarbageCollectionCall!>runtime.gc()<!>
    }

    fun heldInLocal() {
        val rt = Rt.getRuntime()
        <!ExplicitGarbageCollectionCall!>rt.runFinalization()<!>
    }

    fun safeReceiver() {
        // Go misses the safe call; the resolved JDK Runtime method still explicitly collects when non-null.
        <!ExplicitGarbageCollectionCall!>Rt.getRuntime()?.gc()<!>
    }

    fun implicitReceiver() {
        with(Rt.getRuntime()) {
            <!ExplicitGarbageCollectionCall!>gc()<!>
        }
    }
}
