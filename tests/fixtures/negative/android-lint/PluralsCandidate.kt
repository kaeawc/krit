package test

fun formatItems(count: Int): String {
    return resources.getQuantityString(R.plurals.items, count, count)
}
