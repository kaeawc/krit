// RENDER_DIAGNOSTICS_FULL_TEXT
// A factory function named NoSuchElementException that returns the stdlib
// exception. Go rejects every throw of NoSuchElementException in a file that
// declares anything with that name, so it reports both iterators below; both
// bodies throw a java.util.NoSuchElementException. Go's same-file check needs
// this declaration, so the case has a file of its own.
package factory

fun NoSuchElementException(index: Int): NoSuchElementException = NoSuchElementException("index $index")

class Plain(private val items: List<Int>) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    // Go reports this because the file declares a function named
    // NoSuchElementException; FIR is correct to drop it because the call
    // constructs java.util.NoSuchElementException (the factory takes an Int).
    override fun next(): Int {
        if (!hasNext()) throw NoSuchElementException()
        return items[index++]
    }
}

class ThroughFactory(private val items: List<Int>) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    // Go reports this because the call resolves to the local factory; FIR is
    // correct to drop it because the factory returns a NoSuchElementException,
    // and that is what the body throws.
    override fun next(): Int {
        if (!hasNext()) throw NoSuchElementException(index)
        return items[index++]
    }
}
