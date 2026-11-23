// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24, 28, 32
// Negatives Go reports: the receivers are project declarations, not the JDK classes, so no collector runs. Go
// matches the call text `System.gc()` and `Runtime.getRuntime().gc()` and reports both.
package test

object System {
    fun gc() {
        println("pretend gc")
    }
}

class Runtime {
    fun gc() {
        println("pretend gc")
    }

    companion object {
        fun getRuntime(): Runtime = Runtime()
    }
}

fun localSystem() {
    System.gc()
}

fun localRuntime() {
    Runtime.getRuntime().gc()
}

fun qualifiedJdkIsStillReported() {
    <!ExplicitGarbageCollectionCall!>java.lang.System.gc()<!>
}
