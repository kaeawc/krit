package test

fun formatItems(count: Int): String {
    if (count == 1) {
        return getString(R.string.single_item)
    }
    return getString(R.string.multiple_items)
}
