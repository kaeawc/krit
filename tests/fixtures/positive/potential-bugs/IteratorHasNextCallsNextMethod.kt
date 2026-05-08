package fixtures.positive.potentialbugs

class IteratorHasNextCallsNextMethod<T>(private val items: Iterator<T>) : Iterator<T> {
    override fun hasNext(): Boolean {
        return items.next() != null
    }

    override fun next(): T = items.next()
}
