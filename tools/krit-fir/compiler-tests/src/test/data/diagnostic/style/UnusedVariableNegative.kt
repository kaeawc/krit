// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives Go also leaves alone: every variable is referred to somewhere.
package test.unusedvariablenegative

data class Point(val x: Int, val y: Int)

fun reads() {
    val used = 1
    println(used)
}

fun writes(flag: Boolean) {
    var written = 0
    written = 5
    var counter = 0
    counter++
    var added = 0
    added += 2
    var inLambda = 0
    run { inLambda = 1 }
    var inBranch = 0
    if (flag) {
        inBranch = 1
    }
}

fun templates() {
    val name = "x"
    println("hello $name")
    val braced = 1
    println("value ${braced + 1}")
}

fun references(text: String) {
    val receiver = text
    val callable = receiver::length
    println(callable())
    val named = 1
    println(message = named)
}

fun captured() {
    val captured = 4
    val lambda = { captured }
    println(lambda())
    val local = 1
    fun inner() = local
    println(inner())
}

fun deferredAssignment(flag: Boolean) {
    val late: Int
    late = if (flag) 1 else 2
    println(late)
    val neverAssigned: Int
}

fun allowedNames() {
    val ignored = 1
}

fun destructuring(point: Point, points: List<Point>) {
    val (a, b) = point
    println("a=$a b=${b}")
    for ((k, v) in points) {
        println(k + v)
    }
    for ((_, y) in points) {
        println(y)
    }
}

fun loopsAndParameters(points: List<Point>, unusedParameter: Int) {
    for (item in points) {
        println("body")
    }
    for (index in 0 until 3) {
        println("body")
    }
    points.forEach { unusedLambdaParameter -> println("body") }
    points.map { (unusedX, usedY) -> usedY }
    try {
        println("body")
    } catch (unusedCatch: Exception) {
        println("body")
    }
}

fun whenSubject(x: Int, o: Any?) {
    when (val subject = x) {
        1 -> println("one")
        else -> println("other")
    }
    when (val typed = o) {
        is String -> println("string")
        else -> println("other")
    }
}

fun smartCast(o: Any) {
    val value = o
    if (value is String) {
        println(value.length)
    }
}

class Holder {
    val memberProperty = 1
    private val privateMember = 2
    companion object {
        const val CONSTANT = 3
        val shared = "s"
    }
}

val topLevelProperty = 5

fun sameNameDifferentScope(memberLike: Int) {
    val other = 1
    run {
        val other = 2
        println(other)
    }
    println(other)
}

fun usedInInitializerOfLaterVariable() {
    val first = 1
    val second = first + 1
    println(second)
}

fun usedAsArgumentOfConstructor() {
    val x = 1
    val y = 2
    println(Point(x, y))
}
