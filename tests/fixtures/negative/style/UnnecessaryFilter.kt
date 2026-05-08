package fixtures.negative.style

fun getFirstPositive(list: List<Int>): Int {
    return list.first { it > 0 }
}

fun getFirstOrNullPositive(list: List<Int>): Int? {
    return list.firstOrNull { it > 0 }
}

fun countPositive(list: List<Int>): Int {
    return list.count { it > 0 }
}

fun anyPositive(list: List<Int>): Boolean {
    return list.any { it > 0 }
}

fun nonePositive(list: List<Int>): Boolean {
    return list.none { it > 0 }
}

fun filterThenMap(list: List<Int>): List<Int> {
    return list.filter { it > 0 }.map { it * 2 }
}
