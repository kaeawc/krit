// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 13, 17, 21, 25, 26, 30, 36, 40
// Positives: calls to the JDK collector entry points, spelled as the Go rule matches them, plus the qualified and
// multi-line spellings. Each finding sits on the line where the whole call starts.
package test

class Cleaner {
    fun system() {
        <!ExplicitGarbageCollectionCall!>System.gc()<!>
    }

    fun runtime() {
        <!ExplicitGarbageCollectionCall!>Runtime.getRuntime().gc()<!>
    }

    fun systemFinalization() {
        <!ExplicitGarbageCollectionCall!>System.runFinalization()<!>
    }

    fun runtimeFinalization() {
        <!ExplicitGarbageCollectionCall!>Runtime.getRuntime().runFinalization()<!>
    }

    fun qualified() {
        <!ExplicitGarbageCollectionCall!>java.lang.System.gc()<!>
        <!ExplicitGarbageCollectionCall!>java.lang.Runtime.getRuntime().gc()<!>
    }

    fun multiLine() {
        <!ExplicitGarbageCollectionCall!>Runtime<!>
            .getRuntime()
            .gc()
    }

    fun spacedOut() {
        <!ExplicitGarbageCollectionCall!>Runtime.getRuntime( ).gc( )<!>
    }

    fun inLambda(block: (() -> Unit) -> Unit) {
        block { <!ExplicitGarbageCollectionCall!>System.gc()<!> }
    }
}
