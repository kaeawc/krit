// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// containsKey "guards" Go accepts that do not prove the key is present. Go
// skips an access in a then branch when a `map.containsKey(key)` call appears
// anywhere in the condition with an even number of `!` above it and no
// nested `||` in between (odd `!` and no nested `&&` for an else branch or an
// early return). FIR only looks through `!`, `&&`, `||` and comparisons with
// a boolean literal, and only where the step is sound, so it reports these
// accesses: each one can throw.
package test

fun check(condition: Boolean): Boolean = condition

// Go skips this: it ignores `== false`. The key is absent here.
fun comparedWithFalse(map: Map<String, Int>, key: String): Int {
    if (map.containsKey(key) == false) {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
    return 0
}

// Go skips this: the containsKey result is only an argument of another call.
fun argumentOfCall(map: Map<String, Int>, key: String): Int {
    if (check(!map.containsKey(key)) || check(map.containsKey(key))) {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
    return 0
}

// Go skips this: two `!` make an even count, but `!(a && !c)` is `!a || c`,
// which does not prove c.
fun negatedConjunction(map: Map<String, Int>, key: String, enabled: Boolean): Int {
    if (!(enabled && !map.containsKey(key))) {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
    return 0
}

// Go skips this: the call sits in an if expression inside the condition,
// which is true without it when `enabled` is false.
fun ifInsideCondition(map: Map<String, Int>, key: String, enabled: Boolean): Int {
    if (if (enabled) map.containsKey(key) else true) {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
    return 0
}

// Go skips this: the call sits in a lambda of the condition, which never
// runs for an empty list.
fun insideLambda(map: Map<String, Int>, key: String, flags: List<Boolean>): Int {
    if (flags.all { map.containsKey(key) }) {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
    return 0
}

// Go skips this: it rejects an `||` between the call and the condition, but
// not an `||` that is the condition itself. The branch runs whenever
// `enabled` is true, key or no key.
fun disjunctionThen(map: Map<String, Int>, key: String, enabled: Boolean): Int {
    if (enabled || map.containsKey(key)) {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
    return 0
}

// Go skips this: likewise it does not reject the `&&` that is the whole
// condition of an early return. Execution continues past it whenever
// `enabled` is false, key or no key.
fun earlyConjunction(map: Map<String, Int>, key: String, enabled: Boolean): Int {
    if (enabled && !map.containsKey(key)) return 0
    return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
}

// Go skips this: it does not check the condition's own `&&` for an else
// branch either. The else branch runs whenever `enabled` is false, key or no
// key.
fun conjunctionElseExpression(map: Map<String, Int>, key: String, enabled: Boolean): Int =
    if (enabled && !map.containsKey(key)) 0 else <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>

fun conjunctionElseBlock(map: Map<String, Int>, key: String, enabled: Boolean): Int {
    if (enabled && !map.containsKey(key)) {
        return 0
    } else {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
}

// Go skips these: the infix `or` / `and` are not a disjunction or a
// conjunction to Go, so it never rejects them. The then branch runs whenever
// `enabled` is true, and execution continues past the early return whenever
// `enabled` is false, key or no key.
fun infixOrThen(map: Map<String, Int>, key: String, enabled: Boolean): Int {
    if (enabled or map.containsKey(key)) {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
    return 0
}

fun nestedInfixOrThen(map: Map<String, Int>, key: String, enabled: Boolean, strict: Boolean): Int {
    if (strict && (enabled or map.containsKey(key))) {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
    return 0
}

fun infixAndEarly(map: Map<String, Int>, key: String, enabled: Boolean): Int {
    if (enabled and !map.containsKey(key)) return 0
    return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
}

// Go skips this: the call is the receiver of `let`, which returns its
// lambda's result, here the negation. FIR looks through `also` and `apply`
// only, which return their receiver.
fun receiverOfLet(map: Map<String, Int>, key: String): Int {
    if (map.containsKey(key).let { !it }) {
        return <!MapGetWithNotNullAssertionOperator!>map[key]!!<!>
    }
    return 0
}

// Both skip this sound early return: `!(c == true)` leaves when c is false.
fun earlyNegatedComparison(map: Map<String, Int>, key: String): Int {
    if (!(map.containsKey(key) == true)) return 0
    return map[key]!!
}
