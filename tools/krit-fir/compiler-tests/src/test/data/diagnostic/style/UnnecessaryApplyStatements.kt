// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 29, 41, 47, 55, 119, 123
// Statement shapes and implicit-receiver counts. Markers sit where detekt 1.23.8 (type resolution) reports. Go has no types, so it reports only the explicit-this forms listed in the go-lines header; every other marker is a FIR-only finding.
package test

import kotlin.apply

class B {
    var v = 0
    var name: String = ""
    val list = mutableListOf<Int>()
    fun foo() {}
    fun bar(x: Int) {}
    fun self(): B = this
    fun ret(): Int = 1
    operator fun invoke() {}
    operator fun get(i: Int): Int = i
    operator fun set(i: Int, x: Int) {}
    inner class In { fun hi() {} }
}

fun B.ext() {}
fun topFn() {}
var topVar = 0

fun statementCases(b: B, other: B, local: Int) {
    b.<!UnnecessaryApply!>apply<!> { foo() } // C01 flag call
    b.<!UnnecessaryApply!>apply<!> { v = 1 } // C02 flag assign
    b.<!UnnecessaryApply!>apply<!> { this.foo() } // C03 flag this call
    b.apply { foo(); foo() } // C04 two stmts
    b.apply { } // C05 empty
    b.apply { topFn() } // C06 top-level
    b.apply { topVar = 1 } // C07 top var
    b.apply { local.toString() } // C08 local
    b.<!UnnecessaryApply!>apply<!> { v += 1 } // C09 compound assign
    b.<!UnnecessaryApply!>apply<!> { name = "x" } // C10 assign string
    b.<!UnnecessaryApply!>apply<!> { ext() } // C11 extension on receiver
    b.apply { bar(v) } // C12 two receiver refs
    b.<!UnnecessaryApply!>apply<!> { bar(local) } // C13 receiver + local
    b.apply { other.foo() } // C14 other.foo
    b.<!UnnecessaryApply!>apply<!> { this } // C15 this
    b.<!UnnecessaryApply!>apply<!> { v } // C16 reference
    b.apply { v + 1 } // C17 binary
    b.apply { !list.isEmpty() } // C18 prefix
    b.<!UnnecessaryApply!>apply<!> { list.add(1) } // C19 qualified chain
    b.<!UnnecessaryApply!>apply<!> { list[0] } // C20 array access
    b.<!UnnecessaryApply!>apply<!> { this[0] = 1 } // C21 index assign explicit
    b.<!UnnecessaryApply!>apply<!> { self().foo() } // C22 chain
    b.apply { v++ } // C23 postfix
    b.apply { if (v > 0) foo() } // C24 if
    b.apply { (foo()) } // C25 parenthesized
    b.apply { foo()?.toString() } // C26 safe inside
    b.<!UnnecessaryApply!>apply<!> { list.forEach { println(it) } } // C27 nested lambda no receiver use except list
    b.apply { list.forEach { bar(it) } } // C28 nested lambda uses bar
    b.<!UnnecessaryApply!>apply<!> { this@apply.foo() } // C29 labeled this
    b.<!UnnecessaryApply!>apply<!> { ret().toString() } // C30 call + dot
    b.<!UnnecessaryApply!>apply<!> { ret() } // C31 ret call
    b.apply(fun B.() { foo() }) // C32 anonymous function
    b.apply({ foo() }) // C33 paren lambda
}

fun usedCases(b: B?, b2: B): B {
    val x = b2.apply { foo() } // C34 assigned
    b2.apply { foo() }.foo() // C35 chained
    b?.<!UnnecessaryApply!>apply<!> { foo() } // C36 safe stmt
    b?.<!UnnecessaryApply!>apply<!> { v = 2 } // C37 safe assign
    listOf(1).forEach { b2.<!UnnecessaryApply!>apply<!> { foo() } } // C38 last stmt in Unit lambda
    run { b2.apply { foo() } } // C39 last stmt in value lambda
    if (x.v > 0) b2.apply { foo() } // C40 if without braces
    if (x.v > 0) { b2.<!UnnecessaryApply!>apply<!> { foo() } } // C41 if braces stmt
    val y = if (x.v > 0) { b2.apply { foo() } } else b2 // C42 if expr
    println(b2.apply { foo() }) // C43 argument
    for (i in 0..1) { b2.<!UnnecessaryApply!>apply<!> { foo() } } // C44 for body
    while (false) b2.apply { foo() } // C45 while body no brace
    when (x.v) { 1 -> b2.apply { foo() } else -> {} } // C46 when branch stmt
    try { b2.<!UnnecessaryApply!>apply<!> { foo() } } finally { b2.<!UnnecessaryApply!>apply<!> { foo() } } // C47 try
    b2.<!UnnecessaryApply!>apply<!> { foo() } // C48 second stmt before return
    return b2.apply { foo() } // C49 return
}

fun lastStmt(b: B) {
    b.<!UnnecessaryApply!>apply<!> { foo() } // C50 last in Unit fun
}

fun lastExprBody(b: B) = b.apply { foo() } // C51 expr body

fun lastValue(b: B): B {
    b.<!UnnecessaryApply!>apply<!> { foo() } // C52 not last
    return b
}

fun noRecv(b: B) {
    with(b) {
        apply { foo() } // C53 implicit apply on with receiver
        this.apply { foo() } // C54 this.apply in with
    }
}

class Outer {
    var ov = 0
    fun ofoo() {}
    fun go(b: B) {
        b.apply { ofoo() } // C55 outer member
        b.apply { ov = 1 } // C56 outer prop
        b.apply { this@Outer.ofoo() } // C57 labeled outer this
        b.apply { this@Outer.ov = 2 } // C58 labeled outer this assign
        b.<!UnnecessaryApply!>apply<!> { foo() } // C59 inner recv call
        b.apply { b.foo() } // C60 explicit other recv
        b.<!UnnecessaryApply!>apply<!> { with(this) { foo() } } // C61 nested with
        b.<!UnnecessaryApply!>apply<!> { also { it.foo() } } // C62 also with it
        b.<!UnnecessaryApply!>apply<!> { run { foo() } } // C63 run nested
        b.<!UnnecessaryApply!>apply<!> { apply { foo() } } // C64 nested apply
        b.apply { object { fun x() = foo() } } // C65 object expr
        b.apply { fun local() { foo() }; local() } // C66 local fun two
    }
}

fun lambdaParam(b: B, foo: Int) {
    b.<!UnnecessaryApply!>apply<!> { this.v = foo } // C67 local named like member
    b.<!UnnecessaryApply!>apply<!> { v = foo } // C68 assign with param
    val v = 3
    b.apply { v.toString() } // C69 shadow local
    b.<!UnnecessaryApply!>apply<!> { this.v.toString() } // C70 explicit this + member
}

fun builders() {
    StringBuilder().<!UnnecessaryApply!>apply<!> { append("x") } // C71 stringbuilder
    StringBuilder().<!UnnecessaryApply!>apply<!> { append("x").append("y") } // C72 chain
    StringBuilder().<!UnnecessaryApply!>apply<!> { length } // C73 property
    StringBuilder().<!UnnecessaryApply!>apply<!> { setLength(0) } // C74 java setter
    StringBuilder().apply { setLength(0); append("a") } // C75 two
    java.util.ArrayList<Int>().<!UnnecessaryApply!>apply<!> { add(1) } // C76 java list
    java.util.ArrayList<Int>().apply { add(size) } // C77 two refs
    mutableListOf(1).<!UnnecessaryApply!>apply<!> { add(2) } // C78 kotlin list
    mutableListOf(1).<!UnnecessaryApply!>apply<!> { println(size) } // C79 println(size)
    Thread().<!UnnecessaryApply!>apply<!> { name = "t" } // C80 java synthetic prop
    Thread().<!UnnecessaryApply!>apply<!> { setName("t") } // C81 java setter
    Thread().<!UnnecessaryApply!>apply<!> { isDaemon = true } // C82 isDaemon
}
