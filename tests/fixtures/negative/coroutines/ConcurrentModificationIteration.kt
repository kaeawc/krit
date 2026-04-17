package test

data class Item(val stale: Boolean)

fun removeWithBulkOperation(items: MutableList<Item>) {
    items.removeAll { it.stale }
}

fun removeWithIterator(items: MutableList<Item>) {
    val iterator = items.iterator()
    while (iterator.hasNext()) {
        if (iterator.next().stale) {
            iterator.remove()
        }
    }
}
