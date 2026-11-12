// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 10, 11, 17, 22, 26, 31, 38, 41, 44, 48, 54, 61x2, 62, 63, 65x2, 71, 73, 77, 85, 90, 94, 95, 96, 97
// Positives: a local variable that nothing refers to.
package test.unusedvariable

data class Point(val x: Int, val y: Int)

fun plain() {
    val <!UnusedVariable!>unused<!> = 1
    var <!UnusedVariable!>unusedVar<!> = 2
    val <!UnusedVariable!>typed<!>: String = "x"
    println("body")
}

class Worker {
    fun run() {
        val <!UnusedVariable!>inMethod<!> = 2
        println("ready")
    }

    init {
        val <!UnusedVariable!>inInit<!> = 1
    }

    constructor(a: Int) {
        val <!UnusedVariable!>inCtor<!> = a
    }

    val prop: Int
        get() {
            val <!UnusedVariable!>inGetter<!> = 2
            return 1
        }
}

fun scopes(flag: Boolean) {
    run {
        val <!UnusedVariable!>inLambda<!> = 1
    }
    if (flag) {
        val <!UnusedVariable!>inIf<!> = 1
    }
    fun local() {
        val <!UnusedVariable!>inLocalFunction<!> = 1
    }
    local()
    val anonymous = fun(): Int {
        val <!UnusedVariable!>inAnonymous<!> = 1
        return 2
    }
    println(anonymous())
    val holder = object {
        fun method() {
            val <!UnusedVariable!>inObject<!> = 1
        }
    }
    println(holder)
}

fun destructuring(point: Point, points: List<Point>) {
    val (<!UnusedVariable!>a<!>, <!UnusedVariable!>b<!>) = point
    val (<!UnusedVariable!>c<!>, _) = point
    val (d, <!UnusedVariable!>e<!>) = point
    println(d)
    for ((<!UnusedVariable!>k<!>, <!UnusedVariable!>v<!>) in points) {
        println("body")
    }
}

fun usedOnlyBySiblingDeclaration() {
    val <!UnusedVariable!>derived<!> = 1 + 2
    val source = 1
    val <!UnusedVariable!>copy<!> = source
}

fun shadowedByInner() {
    val <!UnusedVariable!>shadow<!> = 1
    run {
        val shadow = 2
        println(shadow)
    }
}

fun backticked() {
    val <!UnusedVariable!>`back tick`<!> = 1
}

// The default `allowedNames` is a full match, so a name that only contains an underscore is reported.
fun underscoreInName() {
    val <!UnusedVariable!>has_underscore<!> = 3
}

fun complexInitializers(flag: Boolean) {
    val <!UnusedVariable!>fromIf<!> = if (flag) 1 else 2
    val <!UnusedVariable!>fromTry<!> = try { 1 } catch (e: Exception) { 2 }
    val <!UnusedVariable!>fromLambda<!> = { 1 }
    val <!UnusedVariable!>fromWhen<!> = when { flag -> 1 else -> 2 }
}
