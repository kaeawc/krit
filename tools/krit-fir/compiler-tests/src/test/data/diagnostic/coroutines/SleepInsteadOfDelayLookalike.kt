// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 22
// Names that match the text Thread.sleep but do not resolve to java.lang.Thread.sleep.
package test

class Napper {
    fun sleep(ms: Long) {}
}

object Thread {
    fun sleep(ms: Long) {}
}

class Other {
    object Local {
        fun sleep(ms: Long) {}
    }
}

// Go reports this by the spelling Thread.sleep; the project object Thread is not java.lang.Thread.
suspend fun projectThread() {
    Thread.sleep(10)
}

suspend fun receiver(napper: Napper) {
    napper.sleep(10)
}

suspend fun nested() {
    Other.Local.sleep(10)
}
