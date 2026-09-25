// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23
// A nested class named NoSuchElementException does not shadow the stdlib
// exception outside its own class. Go rejects every throw of
// NoSuchElementException in a file that declares anything with that name,
// wherever it is declared, so it reports the iterator below; FIR resolves the
// call to java.util.NoSuchElementException. Go's same-file check needs this
// declaration, so the case has a file of its own.
package nestedname

class Tree {
    class NoSuchElementException
}

class Walker(private val items: List<Int>) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    // Go reports this because the file declares a nested class named
    // NoSuchElementException; FIR is correct to drop it because the call
    // constructs java.util.NoSuchElementException.
    override fun next(): Int {
        if (!hasNext()) throw NoSuchElementException("exhausted")
        return items[index++]
    }
}
