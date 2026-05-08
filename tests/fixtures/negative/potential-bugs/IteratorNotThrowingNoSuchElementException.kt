package fixtures.negative.potentialbugs

class IteratorNotThrowingNoSuchElementException<T>(
    private val items: List<T>,
    private var index: Int = 0
) : Iterator<T> {
    override fun hasNext(): Boolean = index < items.size

    override fun next(): T {
        if (!hasNext()) throw NoSuchElementException()
        return items[index++]
    }
}
