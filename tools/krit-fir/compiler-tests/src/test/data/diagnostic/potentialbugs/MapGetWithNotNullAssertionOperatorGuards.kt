// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 123, 130, 139, 145, 152, 158, 163, 169, 173, 180, 188, 195, 200, 204, 212, 219, 226, 234
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
