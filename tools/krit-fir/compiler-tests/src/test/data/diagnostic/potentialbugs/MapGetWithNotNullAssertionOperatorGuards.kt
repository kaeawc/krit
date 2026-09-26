// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 171, 178, 187, 193, 200, 206, 211, 217, 221, 228, 236, 243, 248, 252, 260, 267, 274, 282, 288, 297, 304, 311
// containsKey guards: the accesses Go and FIR both skip because a
// `map.containsKey(key)` check proves the key is present, and the look-alike
// guards both still report.
package test

fun thenBranch(map: Map<String, Int>, key: String): Int {
    if (map.containsKey(key)) {
        return map[key]!!
    }
    return 0
}

fun thenBranchCall(map: Map<String, Int>, key: String): Int {
    if (map.containsKey(key)) return map.get(key)!!
    return 0
}

fun conjunction(map: Map<String, Int>, key: String, enabled: Boolean): Int {
    if (enabled && map.containsKey(key)) {
        return map[key]!!
    }
    return 0
}

fun elseBranch(map: Map<String, Int>, key: String): Int {
    if (!map.containsKey(key)) {
        return 0
    } else {
        return map[key]!!
    }
}

fun elseDisjunction(map: Map<String, Int>, key: String, disabled: Boolean): Int {
    return if (disabled || !map.containsKey(key)) 0 else map[key]!!
}

fun nestedIf(map: Map<String, Int>, key: String, enabled: Boolean): Int {
    if (map.containsKey(key)) {
        if (enabled) {
            return map[key]!!
        }
    }
    return 0
}

fun earlyReturn(map: Map<String, Int>, key: String): Int {
    if (!map.containsKey(key)) return 0
    return map[key]!!
}

fun earlyReturnBlock(map: Map<String, Int>, key: String): Int {
    if (!map.containsKey(key)) {
        return 0
    }
    val value = map[key]!!
    return value
}

fun earlyThrow(map: Map<String, Int>, key: String): Int {
    if (!map.containsKey(key)) throw IllegalStateException(key)
    return map[key]!!
}

fun earlyDisjunction(map: Map<String, Int>, key: String, disabled: Boolean): Int {
    if (disabled || !map.containsKey(key)) return 0
    return map[key]!!
}

fun earlyIfElseLeaves(map: Map<String, Int>, key: String, strict: Boolean): Int {
    if (!map.containsKey(key)) {
        if (strict) throw IllegalStateException(key) else return 0
    }
    return map[key]!!
}

fun loop(map: Map<String, Int>, keys: List<String>): Int {
    var sum = 0
    for (key in keys) {
        if (!map.containsKey(key)) continue
        sum += map[key]!!
    }
    return sum
}

fun comparedWithTrue(map: Map<String, Int>, key: String): Int {
    if (map.containsKey(key) == true) {
        return map[key]!!
    }
    return 0
}

fun anonymousFunction(map: Map<String, Int>, key: String): Int {
    if (map.containsKey(key)) {
        val read = fun(): Int { return map[key]!! }
        return read()
    }
    return 0
}

fun elseIfChain(map: Map<String, Int>, key: String, enabled: Boolean): Int {
    return if (!map.containsKey(key)) {
        0
    } else if (enabled) {
        map[key]!!
    } else {
        1
    }
}

fun parenthesizedReceiver(map: Map<String, Int>, key: String): Int {
    if ((map).containsKey((key))) {
        return (map)[key]!!
    }
    return 0
}

// The infix `and` / `or` on Boolean prove the key as `&&` / `||` do. Go does
// not treat them as a conjunction or a disjunction, so it accepts them too.
fun infixAnd(map: Map<String, Int>, key: String, enabled: Boolean): Int {
    if (map.containsKey(key) and enabled) {
        return map[key]!!
    }
    return 0
}

fun infixOrEarly(map: Map<String, Int>, key: String, disabled: Boolean): Int {
    if (!map.containsKey(key) or disabled) return 0
    return map[key]!!
}

fun infixOrElse(map: Map<String, Int>, key: String, disabled: Boolean): Int =
    if (disabled or !map.containsKey(key)) 0 else map[key]!!

fun negatedInfixAndElse(map: Map<String, Int>, key: String, enabled: Boolean): Int =
    if (!(map.containsKey(key) and enabled)) 0 else map[key]!!

// A true `||` whose operands both prove the key proves it. Go does not check
// the condition's own top-level `||`.
fun disjunctionOfGuards(map: Map<String, Int>, key: String, a: Boolean, b: Boolean): Int {
    if ((map.containsKey(key) && a) || (map.containsKey(key) && b)) {
        return map[key]!!
    }
    return 0
}

// A false `&&` whose operands both prove the key proves it.
fun earlyConjunctionOfGuards(map: Map<String, Int>, key: String, a: Boolean, b: Boolean): Int {
    if ((!map.containsKey(key) || a) && (!map.containsKey(key) || b)) return 0
    return map[key]!!
}

// `also` and `apply` return their receiver.
fun alsoGuard(map: Map<String, Int>, key: String): Int {
    if (map.containsKey(key).also { println(it) }) {
        return map[key]!!
    }
    return 0
}

fun applyEarly(map: Map<String, Int>, key: String): Int {
    if (!map.containsKey(key).apply { println(this) }) return 0
    return map[key]!!
}

// Still reported: the guard does not cover the access.

fun otherKey(map: Map<String, Int>, key: String, other: String): Int {
    if (map.containsKey(key)) {
        return <!MapGetWithNotNullAssertionOperator!>map[other]!!<!>
    }
    return 0
}

fun otherMap(map: Map<String, Int>, other: Map<String, Int>, key: String): Int {
    if (map.containsKey(key)) {
        return <!MapGetWithNotNullAssertionOperator!>other[key]!!<!>
    }
    return 0
}

fun wrongBranch(map: Map<String, Int>, key: String): Int {
    if (map.containsKey(key)) {
        return 0
    } else {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
}

fun negatedThen(map: Map<String, Int>, key: String): Int {
    if (!map.containsKey(key)) {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
    return 0
}

fun nestedDisjunctionThen(map: Map<String, Int>, key: String, enabled: Boolean, strict: Boolean): Int {
    if (strict && (enabled || map.containsKey(key))) {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
    return 0
}

fun conjunctionElse(map: Map<String, Int>, key: String, enabled: Boolean): Int {
    return if (enabled && map.containsKey(key)) 0 else <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
}

fun safeContainsKey(map: Map<String, Int>, key: String): Int {
    if (map?.containsKey(key) == true) {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
    return 0
}

fun sameConditionRightOperand(map: Map<String, Int>, key: String): Boolean =
    map.containsKey(key) && <!MapGetWithNotNullAssertionOperator!>map[key]!!<!> > 0

fun inLambda(map: Map<String, Int>, key: String): List<Int> {
    if (map.containsKey(key)) {
        return listOf(1).map { <!MapGetWithNotNullAssertionOperator!>map[key]!!<!> + it }
    }
    return emptyList()
}

fun inLocalFunction(map: Map<String, Int>, key: String): Int {
    if (map.containsKey(key)) {
        fun read(): Int = <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
        return read()
    }
    return 0
}

fun earlyReturnWithElse(map: Map<String, Int>, key: String): Int {
    if (!map.containsKey(key)) return 0 else println(key)
    return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
}

fun earlyNoExit(map: Map<String, Int>, key: String): Int {
    if (!map.containsKey(key)) {
        println(key)
    }
    return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
}

fun earlyNotNegated(map: Map<String, Int>, key: String): Int {
    if (map.containsKey(key)) return 0
    return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
}

fun earlyAfterUse(map: Map<String, Int>, key: String): Int {
    val value = <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    if (!map.containsKey(key)) return 0
    return value
}

fun earlyInOuterBlock(map: Map<String, Int>, key: String, enabled: Boolean): Int {
    if (!map.containsKey(key)) return 0
    if (enabled) {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
    return 0
}

fun earlyLeavesWithError(map: Map<String, Int>, key: String): Int {
    if (!map.containsKey(key)) error("missing $key")
    return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
}

fun earlyTrailingComment(map: Map<String, Int>, key: String): Int {
    if (!map.containsKey(key)) {
        return 0 // missing
    }
    return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
}

fun earlyCommentOnOwnLine(map: Map<String, Int>, key: String): Int {
    if (!map.containsKey(key)) {
        return 0
        // missing
    }
    return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
}

// A nested `||` proves nothing to Go, even when both operands prove the key.
fun nestedDisjunctionOfGuards(map: Map<String, Int>, key: String, a: Boolean, b: Boolean, strict: Boolean): Int {
    if (strict && ((map.containsKey(key) && a) || (map.containsKey(key) && b))) {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
    return 0
}

// A comment ahead of the condition hides it from Go, which takes the first
// named child of the `if` as its condition.
fun commentBeforeParen(map: Map<String, Int>, key: String): Int {
    if /* present */ (map.containsKey(key)) {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
    return 0
}

fun commentInsideParen(map: Map<String, Int>, key: String): Int {
    if (/* present */ map.containsKey(key)) {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
    return 0
}

fun commentBeforeParenEarly(map: Map<String, Int>, key: String): Int {
    if /* absent */ (!map.containsKey(key)) return 0
    return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
}
