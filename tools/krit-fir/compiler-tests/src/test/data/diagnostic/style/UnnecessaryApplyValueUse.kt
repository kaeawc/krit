// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 52, 53, 54, 94
// Positions where the apply value is used or dropped, and statement shapes. Markers sit where detekt 1.23.8 (type resolution) reports. Go has no types, so it reports only the explicit-this forms listed in the go-lines header; every other marker is a FIR-only finding.
package test

import kotlin.concurrent.thread

class Box {
    var v = 0
    val list = mutableListOf<Int>()
    fun foo() {}
    fun bar(x: Int) {}
    fun withArgs(a: Int, f: () -> Int) {}
    fun self(): Box = this
    operator fun get(i: Int): Int = i
    operator fun set(i: Int, x: Int) {}
    val Box.extProp: Int get() = 1
}

enum class Color { RED, GREEN }

fun <T : Box> generic(t: T) {
    t.<!UnnecessaryApply!>apply<!> { foo() } // G01 generic receiver
    t.<!UnnecessaryApply!>apply<!> { v = 2 } // G02 generic assign
}

fun nestedApply(a: Box, b: Box) {
    a.apply { b.<!UnnecessaryApply!>apply<!> { foo() } } // G03 outer
    a.apply { b.<!UnnecessaryApply!>apply<!> { this.foo() } } // G04 nested explicit
    a.apply { also { foo() } } // G05 also then foo
    a.<!UnnecessaryApply!>apply<!> { run { v } } // G06 run v
    a.apply { with(b) { foo() } } // G07 with other
}

fun shapes(a: Box) {
    a.<!UnnecessaryApply!>apply<!> { withArgs(1) { 2 } } // G08 trailing lambda call
    a.<!UnnecessaryApply!>apply<!> { withArgs(1, { 2 }) } // G09 lambda arg
    a.apply { this.v++ } // G10 postfix
    a.apply { v++ } // G11 postfix bare
    a.apply { v != 0 } // G12 comparison
    a.apply { v to 1 } // G13 infix
    a.apply { "${v}" } // G14 template
    a.apply { if (v > 0) foo() else bar(1) } // G15 if
    a.apply { when (v) { 0 -> foo() else -> bar(1) } } // G16 when
    a.apply { val x = v } // G17 declaration
    a.apply { return@apply } // G18 jump
    a.<!UnnecessaryApply!>apply<!> { foo() ; } // G19 semi
    a.<!UnnecessaryApply!>apply<!> { self().self().foo() } // G20 chain
    a.<!UnnecessaryApply!>apply<!> { list.size } // G21 list.size
    a.<!UnnecessaryApply!>apply<!> { list.map { it + 1 } } // G22 map
    a.apply { list.forEach { bar(it) } } // G23 two refs
    a.<!UnnecessaryApply!>apply<!> { this.list.forEach { println(it) } } // G24 this.list
    a.<!UnnecessaryApply!>apply<!> { this[0] } // G25 this get
    a.<!UnnecessaryApply!>apply<!> { this.list[0] = 1 } // G26 list set
    a.<!UnnecessaryApply!>apply<!> { list += 1 } // G27 plus assign operator
    a.<!UnnecessaryApply!>apply<!> { v -= 1 } // G28 minus assign
    a.<!UnnecessaryApply!>apply<!> { v = v + 1 } // G29 self rhs
    a.apply { (v) } // G30 paren ref
    a.apply { v as Int } // G31 cast
    a.apply { v!! } // G32 not null
    a.apply { Color.RED } // G33 enum entry
    a.<!UnnecessaryApply!>apply<!> { println(v) } // G34 println(v)
    a.<!UnnecessaryApply!>apply<!> { requireNotNull(list) } // G35 requireNotNull
    a.apply { error("x") } // G36 error
    a.apply { TODO() } // G37 TODO
}

fun uses(a: Box, flag: Boolean, c: Color): Box? {
    thread { a.<!UnnecessaryApply!>apply<!> { foo() } } // G40 thread lambda
    buildList<Int> { a.<!UnnecessaryApply!>apply<!> { foo() } } // G41 buildList lambda
    lazy { a.apply { foo() } } // G42 lazy
    synchronized(a) { a.apply { foo() } } // G43 synchronized
    runCatching { a.apply { foo() } } // G44 runCatching
    listOf(1).map { a.apply { foo() } } // G45 map
    listOf(1).onEach { a.<!UnnecessaryApply!>apply<!> { foo() } } // G46 onEach
    listOf(1).forEach { if (it > 0) { a.<!UnnecessaryApply!>apply<!> { foo() } } } // G47 if in forEach
    listOf(1).forEach { when (it) { 0 -> { a.<!UnnecessaryApply!>apply<!> { foo() } } else -> {} } } // G48 when in forEach
    val x: Box? = null
    val y = x ?: a.apply { foo() } // G49 elvis
    x?.let { a.apply { foo() } } // G50 let
    when (c) { Color.RED -> { a.<!UnnecessaryApply!>apply<!> { foo() } } Color.GREEN -> { a.<!UnnecessaryApply!>apply<!> { foo() } } } // G51 exhaustive enum when stmt
    val z = when (c) { Color.RED -> { a.apply { foo() } } Color.GREEN -> { a.apply { foo() } } } // G52 exhaustive when value
    if (flag) { a.<!UnnecessaryApply!>apply<!> { foo() } } else { a.<!UnnecessaryApply!>apply<!> { foo() } } // G53 if else stmt
    val w = if (flag) { a.apply { foo() } } else { a } // G54 if else value
    try { a.<!UnnecessaryApply!>apply<!> { foo() } } catch (e: IllegalStateException) { a.<!UnnecessaryApply!>apply<!> { foo() } } finally { a.<!UnnecessaryApply!>apply<!> { foo() } } // G55 try
    val t = try { a.apply { foo() } } catch (e: IllegalStateException) { a } // G56 try value
    do { a.<!UnnecessaryApply!>apply<!> { foo() } } while (false) // G57 do while
    a.<!UnnecessaryApply!>apply<!> { foo() }; a.<!UnnecessaryApply!>apply<!> { foo() } // G58 two on a line
    return a.apply { foo() } // G59 return
}

fun labels(a: Box) {
    a.<!UnnecessaryApply!>apply<!> outer@{ foo() } // G60 labelled lambda
    a.<!UnnecessaryApply!>apply<!> { this@apply.foo() } // G61 this@apply
    listOf(1).forEach lit@{ a.<!UnnecessaryApply!>apply<!> { foo() } } // G62 labelled forEach
    a.also { it.<!UnnecessaryApply!>apply<!> { foo() } } // G63 it.apply
    a.let { it.apply { foo() } } // G64 let it.apply
}

fun Box.receiverFun() {
    apply { foo() } // G65 implicit apply
    this.<!UnnecessaryApply!>apply<!> { foo() } // G66 this.apply
    this@receiverFun.<!UnnecessaryApply!>apply<!> { foo() } // G67 labeled this apply
    self().<!UnnecessaryApply!>apply<!> { foo() } // G68 self apply
    list.<!UnnecessaryApply!>apply<!> { add(1) } // G69 list apply
}

class Outer {
    var o = 0
    fun of() {}
    fun go(a: Box) {
        a.apply { of() } // G70 outer member
        a.apply { this@Outer.of() } // G71 labeled outer
        a.apply { o = v } // G72 outer assign inner read
        a.<!UnnecessaryApply!>apply<!> { v = o } // G73 inner assign outer read
        a.apply { foo() ; of() } // G74 two
        a.apply { object : Runnable { override fun run() { foo() } } } // G75 object expr
    }
}
