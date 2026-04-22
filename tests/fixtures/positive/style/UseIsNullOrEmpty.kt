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

fun checkCollectionSize(items: Collection<String>?) {
    if (items == null || items.size == 0) {
        println("empty")
    }
}

fun checkMultiline(items: List<String>?) {
    if (items == null ||
        items.isEmpty()
    ) {
        println("empty")
    }
}

class Holder(private val text: String?) {
    fun checkThis() {
        if (this.text == null || this.text.isEmpty()) {
            println("empty")
        }
    }
}
