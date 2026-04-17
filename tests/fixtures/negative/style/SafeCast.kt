package style

fun example(x: Any) {
    val s = x as? String
    println(s ?: "not a string")
}

// Different types in is-check vs cast (cast in else branch)
fun differentTypesElseBranch(x: Any) {
    if (x is String) {
        println(x.length)
    } else {
        val n = x as Number
        println(n)
    }
}

// Different variables in is-check vs cast
fun differentVariables(x: Any, y: Any) {
    if (x is String) {
        val n = y as Number
        println(n)
    }
}

// Cast in else branch with same type (is-check does NOT apply in else)
fun castInElseBranch(x: Any) {
    if (x is String) {
        println(x.length)
    } else {
        val s = x as String
        println(s)
    }
}
