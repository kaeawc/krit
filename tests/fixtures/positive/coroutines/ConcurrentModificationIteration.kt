package test

data class Item(val stale: Boolean)

fun removeWhileIterating(items: MutableList<Item>) {
    for (item in items) {
        if (item.stale) {
            items.remove(item)
        }
    }
}

fun addWhileIterating(items: MutableList<Item>, fresh: Item) {
    for (item in items) {
        if (!item.stale) {
            items.add(fresh)
        }
    }
}
