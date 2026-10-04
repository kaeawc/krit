// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Reads Go and FIR both leave alone: a Nothing-typed value (`return`,
// `throw`, `error()`, `TODO()`, `null`) is a subtype of every stream type but
// never is a FileInputStream.
package test

import java.io.InputStream

class Wrap(private val source: InputStream, private val onClose: (() -> Unit)?) : InputStream() {
    override fun read(): Int = source.read()
}

class NothingValues {
    fun resourceOrReturn(): Int {
        val input = javaClass.getResourceAsStream("x") ?: return -1
        return input.read()
    }

    fun elvisThrow(stream: InputStream?): Int {
        val input = stream ?: throw IllegalStateException()
        return input.read()
    }

    fun elvisReturnReceiver(stream: InputStream?): Int = (stream ?: return -1).read()

    fun nullBranch(stdin: Boolean): Int? {
        val input: InputStream? = if (stdin) System.`in` else null
        return input?.read()
    }

    fun errorBranch(kind: String): Int = (
        when (kind) {
            "in" -> System.`in`
            else -> error("bad")
        }
        ).read()

    fun todoBranch(stdin: Boolean): Int = (if (stdin) System.`in` else TODO()).read()

    fun nullArgument(): Int = Wrap(System.`in`, null).read()
}
