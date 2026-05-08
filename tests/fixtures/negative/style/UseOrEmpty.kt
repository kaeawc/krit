package fixtures.negative.style

fun getItems(x: List<String>?): List<String> {
    return x.orEmpty()
}

fun getNonEmpty(x: List<String>?): List<String> {
    return x ?: listOf("default")
}

fun getMutable(x: MutableList<String>?): MutableList<String> {
    return x ?: mutableListOf()
}

fun getIntArray(x: IntArray?): IntArray {
    return x ?: intArrayOf()
}

fun getDefault(x: String?): String {
    return x ?: "default"
}

fun elvisReturn(x: String?): String {
    return x ?: return "fallback"
}

fun elvisThrow(x: String?): String {
    return x ?: throw IllegalStateException()
}
