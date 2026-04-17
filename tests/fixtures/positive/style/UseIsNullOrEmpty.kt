package fixtures.positive.style

fun check(x: String?) {
    if (x == null || x.isEmpty()) {
        println("nothing")
    }
}

fun checkCountZero(x: List<Int>?) {
    if (x == null || x.count() == 0) {
        println("empty")
    }
}

fun checkSizeZero(x: List<Int>?) {
    if (x == null || x.size == 0) {
        println("empty")
    }
}

fun checkLengthZero(x: String?) {
    if (x == null || x.length == 0) {
        println("empty")
    }
}

fun checkEmptyString(x: String?) {
    if (x == null || x == "") {
        println("empty")
    }
}
