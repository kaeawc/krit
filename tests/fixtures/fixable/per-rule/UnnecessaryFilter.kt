package fixtures.positive.style

fun getFirstPositive(list: List<Int>): Int {
    return list.filter { it > 0 }.first()
}

fun getFirstOrNullPositive(list: List<Int>): Int? {
    return list.filter { it > 0 }.firstOrNull()
}

fun getLastPositive(list: List<Int>): Int {
    return list.filter { it > 0 }.last()
}

fun getLastOrNullPositive(list: List<Int>): Int? {
    return list.filter { it > 0 }.lastOrNull()
}

fun getSinglePositive(list: List<Int>): Int {
    return list.filter { it > 0 }.single()
}

fun getSingleOrNullPositive(list: List<Int>): Int? {
    return list.filter { it > 0 }.singleOrNull()
}

fun countPositive(list: List<Int>): Int {
    return list.filter { it > 0 }.count()
}

fun anyPositive(list: List<Int>): Boolean {
    return list.filter { it > 0 }.any()
}

fun nonePositive(list: List<Int>): Boolean {
    return list.filter { it > 0 }.none()
}
