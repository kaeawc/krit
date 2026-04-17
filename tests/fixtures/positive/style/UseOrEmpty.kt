package fixtures.positive.style

fun getItems(x: List<String>?): List<String> {
    return x ?: emptyList()
}

fun getSet(x: Set<Int>?): Set<Int> {
    return x ?: emptySet()
}

fun getMap(x: Map<String, Int>?): Map<String, Int> {
    return x ?: emptyMap()
}

fun getArray(x: Array<Int>?): Array<Int> {
    return x ?: emptyArray()
}

fun getSeq(x: Sequence<Int>?): Sequence<Int> {
    return x ?: emptySequence()
}

fun getStr(x: String?): String {
    return x ?: ""
}

fun getListOf(x: List<String>?): List<String> {
    return x ?: listOf()
}

fun getSetOf(x: Set<Int>?): Set<Int> {
    return x ?: setOf()
}

fun getMapOf(x: Map<String, Int>?): Map<String, Int> {
    return x ?: mapOf()
}

fun getArrayOf(x: Array<Int>?): Array<Int> {
    return x ?: arrayOf()
}

fun getSeqOf(x: Sequence<Int>?): Sequence<Int> {
    return x ?: sequenceOf()
}

fun chainedCall(items: Map<String, List<Int>>?) {
    val result = items?.get("key") ?: emptyList()
}
