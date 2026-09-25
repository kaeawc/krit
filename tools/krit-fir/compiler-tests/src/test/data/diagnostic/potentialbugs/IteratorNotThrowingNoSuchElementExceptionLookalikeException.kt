// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 32
// Local lookalike: this package declares its own NoSuchElementException, which
// shadows kotlin.NoSuchElementException in this file. Throwing it does not
// throw the java.util.NoSuchElementException iterator callers expect, so Go
// (which ignores a NoSuchElementException declared in the same file) and FIR
// both report it.
package lookalikeexception

class NoSuchElementException(message: String? = null) : RuntimeException(message)

class Cursor(private val items: List<Int>) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int {
        if (!hasNext()) throw NoSuchElementException("exhausted")
        return items[index++]
    }
}

// The real one, qualified, satisfies the contract.
class RealCursor(private val items: List<Int>) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    // Go reports this because the call's simple name matches the
    // NoSuchElementException declared in this file; FIR is correct to drop it
    // because the call constructs java.util.NoSuchElementException.
    override fun next(): Int {
        if (!hasNext()) throw java.util.NoSuchElementException("exhausted")
        return items[index++]
    }
}
