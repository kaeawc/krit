// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: other JDK Runtime and System calls, and mentions of the collector that are not calls.
package test

class Probe {
    fun otherCalls() {
        println(Runtime.getRuntime().availableProcessors())
        println(System.currentTimeMillis())
        println(Runtime.getRuntime().totalMemory())
    }

    fun textAndComments() {
        val text = "System.gc() and Runtime.getRuntime().gc()"
        // System.gc()
        println(text)
    }

    fun reference(): () -> Unit = System::gc

    fun ownMethod() {
        gc()
    }

    private fun gc() {
        println("own gc")
    }
}
