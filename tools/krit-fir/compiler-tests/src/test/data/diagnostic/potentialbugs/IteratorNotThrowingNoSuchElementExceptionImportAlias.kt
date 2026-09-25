// RENDER_DIAGNOSTICS_FULL_TEXT
// An import alias of kotlin.collections.Iterator. The alias hides the name
// Iterator in this file, so it needs a file of its own.
package importalias

import kotlin.collections.Iterator as Cursor

// Go misses this because the import alias's name is not an iterator name.
class FromImportAlias(private val items: List<Int>) : Cursor<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int = items[index++]
}

// The alias names an iterator whose next() does throw.
class GuardedImportAlias(private val items: List<Int>) : Cursor<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    override fun next(): Int {
        if (!hasNext()) throw NoSuchElementException()
        return items[index++]
    }
}
