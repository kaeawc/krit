package potentialbugs

import kotlin.system.exitProcess

class Example {
    fun cleanup() {
        exitProcess(0)
    }
}
