package fixtures.negative.style

fun check(x: String?) {
    if (x.isNullOrEmpty()) {
        println("nothing")
    }
}

class Box {
    fun isEmpty() = true
}

class Holder(private val text: String?) {
    fun differentVariables(a: String?, b: String?) {
        if (a == null || b.isEmpty()) {
            println("different")
        }
    }

    fun customIsEmpty(box: Box?) {
        if (box == null || box.isEmpty()) {
            println("custom")
        }
    }

    fun shadowed(text: String?) {
        if (this.text == null || text.isEmpty()) {
            println("shadowed")
        }
    }

    fun unresolved(value: MissingType?) {
        if (value == null || value.isEmpty()) {
            println("unresolved")
        }
    }

    fun commentsAndStrings(x: String?) {
        // x == null || x.isEmpty()
        val text = "x == null || x.isEmpty()"
        println(text)
    }
}
