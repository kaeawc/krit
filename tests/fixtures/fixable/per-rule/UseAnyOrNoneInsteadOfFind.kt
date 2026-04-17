package fixtures.positive.style

fun hasPositive(list: List<Int>): Boolean {
    return list.find { it > 0 } != null
}

fun noPositive(list: List<Int>): Boolean {
    return list.find { it > 0 } == null
}

fun hasFirst(list: List<Int>): Boolean {
    return list.firstOrNull { it > 0 } != null
}

fun noLast(list: List<Int>): Boolean {
    return list.lastOrNull { it > 0 } == null
}
