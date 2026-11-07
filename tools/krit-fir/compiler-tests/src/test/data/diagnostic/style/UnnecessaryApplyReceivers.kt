// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 41
// Which scope owns an unqualified name, plus value-use contexts. Markers sit where detekt 1.23.8 (type resolution) reports. Go has no types, so it reports only the explicit-this forms listed in the go-lines header; every other marker is a FIR-only finding.
package test

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


open class Base { fun baseFoo() {}; var baseV = 0 }
object Single { var sv = 0; fun sfoo() {} }
class Holder { companion object { var cv = 0; fun cfoo() {} } }
class Fn { var cb: () -> Unit = {}; val B.extProp: Int get() = 1 }
var Fn.zz: Int
    get() = 1
    set(value) {}

fun takeUnit(f: () -> Unit) {}
fun takeAny(f: () -> Any) {}
fun <R> takeR(f: () -> R) {}

class Ctx : Base() {
    var v = 5
    fun B.memberExt() {}
    fun go(b: B, o: Ctx) {
        b.<!UnnecessaryApply!>apply<!> { memberExt() } // D01 member extension
        b.apply { baseFoo() } // D02 inherited from outer
        b.apply { baseV = 1 } // D03 inherited prop
        b.<!UnnecessaryApply!>apply<!> { v = 1 } // D04 prop shadows outer v
        b.<!UnnecessaryApply!>apply<!> { (this as B).foo() } // D05 cast this
        b.<!UnnecessaryApply!>apply<!> { listOf(v) } // D06 listOf(v)
        b.apply { "$v" } // D07 template
        b.apply { v ?: 1 } // D08 elvis
        b.apply { v!! } // D09 not-null
        b.apply { -v } // D10 unary
        b.apply { v as Int } // D11 as
        b.<!UnnecessaryApply!>apply<!> { bar(x = 1) } // D12 named arg
        b.apply { this() } // D13 this invoke
        b.<!UnnecessaryApply!>apply<!> { invoke() } // D14 invoke
        b.<!UnnecessaryApply!>apply<!> { v = 1; } // D15 trailing semicolon
        b.<!UnnecessaryApply!>apply<!> { /* c */ foo() /* d */ } // D16 comments
        b.<!UnnecessaryApply!>apply<!> { foo() } // D17 plain
        (b).<!UnnecessaryApply!>apply<!> { foo() } // D18 parenthesized receiver
        b.<!UnnecessaryApply!>apply<!><B> { foo() } // D19 type args
        b.apply { foo() } as Unit // D20 cast of apply
        b.apply { ::foo } // D21 callable ref
        b.apply { this::foo } // D22 bound callable ref
        b.<!UnnecessaryApply!>apply<!> { foo().also { } } // D23 chain with lambda
        b.<!UnnecessaryApply!>apply<!> { foo(); } // D24 semicolon
        b.<!UnnecessaryApply!>apply<!> label@{ foo() } // D25 labeled lambda
    }
    fun more(b: B, f: Fn, local: Int) {
        var lv = 0
        b.apply { lv = 1 } // D27 outer local var
        b.<!UnnecessaryApply!>apply<!> { v = local } // D28 local rhs
        b.apply { baseFoo() } // D29 base
        Single.<!UnnecessaryApply!>apply<!> { sfoo() } // D30 object recv
        Single.<!UnnecessaryApply!>apply<!> { sv = 1 } // D31 object prop
        Holder.<!UnnecessaryApply!>apply<!> { cfoo() } // D32 holder
        Holder.Companion.<!UnnecessaryApply!>apply<!> { cv = 2 } // D33 companion
        f.apply { cb() } // D34 fn property invoke
        f.<!UnnecessaryApply!>apply<!> { zz = 1 } // D35 ext prop on receiver
        f.<!UnnecessaryApply!>apply<!> { cb = {} } // D36 lambda rhs
        b.apply { with(f) { zz = 3 } } // D37 nested with
        b.apply { foo() }.let { } // D38 apply then let
        b.run { apply { foo() } } // D39 apply inside run
    }
    fun unitCases(b: B, flag: Boolean) {
        if (flag) { b.<!UnnecessaryApply!>apply<!> { foo() } } // D40 if last in unit fn
        when (flag) { true -> { b.<!UnnecessaryApply!>apply<!> { foo() } } else -> {} } // D41 when block last
        try { b.<!UnnecessaryApply!>apply<!> { foo() } } catch (e: Exception) { b.<!UnnecessaryApply!>apply<!> { foo() } } // D42 try catch last
        takeUnit { b.<!UnnecessaryApply!>apply<!> { foo() } } // D43 unit lambda
        takeAny { b.apply { foo() } } // D44 any lambda
        takeR { b.apply { foo() } } // D45 generic lambda
        b.also { b.<!UnnecessaryApply!>apply<!> { foo() } } // D46 also lambda
        b.let { b.apply { foo() } } // D47 let lambda
        repeat(2) { b.<!UnnecessaryApply!>apply<!> { foo() } } // D48 repeat lambda
        val f: () -> Unit = { b.<!UnnecessaryApply!>apply<!> { foo() } } // D49 declared unit lambda
        val g = { b.apply { foo() } } // D50 inferred lambda
        val h: () -> B = { b.apply { foo() } } // D51 typed B lambda
        run { if (flag) { b.<!UnnecessaryApply!>apply<!> { foo() } } } // D52 if in run lambda last
        run { if (flag) { b.apply { foo() } } else { b.apply { foo() } } } // D53 if-else in run
        takeUnit { if (flag) b.apply { foo() } } // D54 unbraced if in unit lambda
        takeUnit { if (flag) { b.<!UnnecessaryApply!>apply<!> { foo() } } } // D55 if braced in unit lambda
        Thread { b.<!UnnecessaryApply!>apply<!> { foo() } } // D56 SAM Runnable
        takeUnit { return@takeUnit } // D57 filler
        val z = if (flag) { 1 } else { b.<!UnnecessaryApply!>apply<!> { foo() }; 2 } // D58 not last in if expr
        val w = try { b.<!UnnecessaryApply!>apply<!> { foo() }; 1 } finally { } // D59 not last in try expr
        fun local(): Unit { b.<!UnnecessaryApply!>apply<!> { foo() } } // D60 local fun
        val anon = fun() { b.<!UnnecessaryApply!>apply<!> { foo() } } // D61 anonymous fun
        val ret = fun(): B { return b.apply { foo() } } // D62
        object : Runnable { override fun run() { b.<!UnnecessaryApply!>apply<!> { foo() } } } // D63 object expr member
    }
    init { B().<!UnnecessaryApply!>apply<!> { foo() } } // D64 init block
    val pp: Int get() { B().<!UnnecessaryApply!>apply<!> { foo() }; return 1 } // D65 getter
}

class Sec : Base {
    constructor() : super() { B().<!UnnecessaryApply!>apply<!> { foo() } } // D66 secondary ctor
}
