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

// when-expression with safe cast already (no finding)
fun processWhenSafe(x: Any) {
    val s = x as? String
    println(s ?: "not a string")
}

// when-expression with different types (no finding)
fun processWhenDifferentTypes(x: Any) {
    when (x) {
        is String -> println(x.length)
        else -> {
            val n = x as Number
            println(n)
        }
    }
}

// A cast repeated after an is-check is not reported unless reportRedundantCasts is on
fun redundantCastAfterIsCheck(x: Any) {
    if (x is String) {
        val s = x as String
        println(s)
    }
}

// Other branch values are not the safe cast pattern
fun otherValues(x: Any, y: Any?) {
    val a = if (x is String) x else "default"
    val b = if (x is String) y else null
    val c = if (x is String) x else x
    println("$a $b $c")
}
