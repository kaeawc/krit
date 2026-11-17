package p

import android.annotation.TargetApi
import android.os.Build
import androidx.annotation.RequiresApi

class K3 {
    fun x() = true
    // CASE then-ge23
    fun f1(): Int {
        if (Build.VERSION.SDK_INT >= 23) { if (Build.VERSION.SDK_INT >= 22) println() }
        return 0
    }
    // CASE then-ge23-and-x
    fun f2(): Int {
        if (Build.VERSION.SDK_INT >= 23 && x()) { if (Build.VERSION.SDK_INT >= 22) println() }
        return 0
    }
    // CASE then-x-and-ge23
    fun f3(): Int {
        if (x() && Build.VERSION.SDK_INT >= 23) { if (Build.VERSION.SDK_INT >= 22) println() }
        return 0
    }
    // CASE then-ge23-or-x
    fun f4(): Int {
        if (Build.VERSION.SDK_INT >= 23 || x()) { if (Build.VERSION.SDK_INT >= 22) println() }
        return 0
    }
    // CASE else-lt23-or-x
    fun f5(): Int {
        if (Build.VERSION.SDK_INT < 23 || x()) { println() } else { if (Build.VERSION.SDK_INT >= 22) println() }
        return 0
    }
    // CASE else-lt23-and-x
    fun f6(): Int {
        if (Build.VERSION.SDK_INT < 23 && x()) { println() } else { if (Build.VERSION.SDK_INT >= 22) println() }
        return 0
    }
    // CASE elseif-ge22-after-ge23
    fun f7(): Int {
        if (Build.VERSION.SDK_INT >= 23) { } else if (Build.VERSION.SDK_INT >= 22) println()
        return 0
    }
    // CASE elseif-lt22-after-ge23
    fun f8(): Int {
        if (Build.VERSION.SDK_INT >= 23) { println() } else if (Build.VERSION.SDK_INT < 22) println()
        return 0
    }
    // CASE else-nested-x
    fun f9(): Int {
        if (Build.VERSION.SDK_INT < 23) { println() } else { if (x()) { if (Build.VERSION.SDK_INT >= 22) println() } }
        return 0
    }
    // CASE then-lambda
    fun f10(): Int {
        if (Build.VERSION.SDK_INT >= 23) { run { if (Build.VERSION.SDK_INT >= 22) println() } }
        return 0
    }
    // CASE then-lambda2
    fun f11(): Int {
        if (Build.VERSION.SDK_INT >= 23) { val f = { if (Build.VERSION.SDK_INT >= 22) println() } }
        return 0
    }
    // CASE then-localfun
    fun f12(): Int {
        if (Build.VERSION.SDK_INT >= 23) { fun loc() { if (Build.VERSION.SDK_INT >= 22) println() } }
        return 0
    }
    // CASE then-anon
    fun f13(): Int {
        if (Build.VERSION.SDK_INT >= 23) { object : Any() { fun m() { if (Build.VERSION.SDK_INT >= 22) println() } } }
        return 0
    }
    // CASE then-for
    fun f14(): Int {
        if (Build.VERSION.SDK_INT >= 23) { for (i in 0..1) { if (Build.VERSION.SDK_INT >= 22) println() } }
        return 0
    }
    // CASE then-notlt23
    fun f15(): Int {
        if (!(Build.VERSION.SDK_INT < 23)) { if (Build.VERSION.SDK_INT >= 22) println() }
        return 0
    }
    // CASE else-notge23
    fun f16(): Int {
        if (!(Build.VERSION.SDK_INT >= 23)) { println() } else { if (Build.VERSION.SDK_INT >= 22) println() }
        return 0
    }
    // CASE when-then
    fun f17(): Int {
        when { Build.VERSION.SDK_INT >= 23 -> { if (Build.VERSION.SDK_INT >= 22) println() }; else -> {} }
        return 0
    }
    // CASE when-else
    fun f18(): Int {
        when { Build.VERSION.SDK_INT < 23 -> {}; else -> { if (Build.VERSION.SDK_INT >= 22) println() } }
        return 0
    }
    // CASE when-second
    fun f19(): Int {
        when { x() -> {}; Build.VERSION.SDK_INT >= 23 -> { if (Build.VERSION.SDK_INT >= 22) println() } }
        return 0
    }
    // CASE when-cond-after-lt23
    fun f20(): Int {
        when { Build.VERSION.SDK_INT < 23 -> {}; Build.VERSION.SDK_INT >= 22 -> {} }
        return 0
    }
    // CASE ifexpr-then
    fun f21(): Int {
        val v = if (Build.VERSION.SDK_INT >= 23) { if (Build.VERSION.SDK_INT >= 22) println(); 1 } else 2
        return 0
    }
    // CASE while-then
    fun f22(): Int {
        while (Build.VERSION.SDK_INT >= 23) { if (Build.VERSION.SDK_INT >= 22) println(); break }
        return 0
    }
    // CASE then-deep
    fun f23(): Int {
        if (Build.VERSION.SDK_INT >= 23) { if (x()) { if (Build.VERSION.SDK_INT >= 22) println() } }
        return 0
    }
    // CASE then-ge23-ge24-ge23
    fun f24(): Int {
        if (Build.VERSION.SDK_INT >= 23) { if (Build.VERSION.SDK_INT >= 24) { if (Build.VERSION.SDK_INT >= 23) println() } }
        return 0
    }
    // CASE then-ge23-lt25-ge24
    fun f25(): Int {
        if (Build.VERSION.SDK_INT >= 23) { if (Build.VERSION.SDK_INT < 25) { if (Build.VERSION.SDK_INT >= 24) println() } }
        return 0
    }
    // CASE then-ge23-lt25-lt26
    fun f26(): Int {
        if (Build.VERSION.SDK_INT >= 23) { if (Build.VERSION.SDK_INT < 25) { if (Build.VERSION.SDK_INT < 26) println() } }
        return 0
    }
    // CASE then-ge23-lt25-ge22
    fun f27(): Int {
        if (Build.VERSION.SDK_INT >= 23) { if (Build.VERSION.SDK_INT < 25) { if (Build.VERSION.SDK_INT >= 22) println() } }
        return 0
    }
    // CASE then-ge23-lt25-ge25
    fun f28(): Int {
        if (Build.VERSION.SDK_INT >= 23) { if (Build.VERSION.SDK_INT < 25) { if (Build.VERSION.SDK_INT >= 25) println() } }
        return 0
    }
    // CASE eq23-then-ge23
    fun f29(): Int {
        if (Build.VERSION.SDK_INT == 23) { if (Build.VERSION.SDK_INT >= 23) println() }
        return 0
    }
    // CASE eq23-then-eq23
    fun f30(): Int {
        if (Build.VERSION.SDK_INT == 23) { if (Build.VERSION.SDK_INT == 23) println() }
        return 0
    }
    // CASE ne23-then-eq23
    fun f31(): Int {
        if (Build.VERSION.SDK_INT != 23) { if (Build.VERSION.SDK_INT == 23) println() }
        return 0
    }
    // CASE ne23-else-eq23
    fun f32(): Int {
        if (Build.VERSION.SDK_INT != 23) { println() } else { if (Build.VERSION.SDK_INT == 23) println() }
        return 0
    }
    // CASE gt22-then-ge23
    fun f33(): Int {
        if (Build.VERSION.SDK_INT > 22) { if (Build.VERSION.SDK_INT >= 23) println() }
        return 0
    }
    // CASE le24-then-lt25
    fun f34(): Int {
        if (Build.VERSION.SDK_INT <= 24) { if (Build.VERSION.SDK_INT < 25) println() }
        return 0
    }
    // CASE then-rev
    fun f35(): Int {
        if (Build.VERSION.SDK_INT >= 23) { if (23 <= Build.VERSION.SDK_INT) println() }
        return 0
    }
    // CASE then-val
    fun f36(): Int {
        if (Build.VERSION.SDK_INT >= 23) { val z = Build.VERSION.SDK_INT >= 22 }
        return 0
    }
    // CASE then-try
    fun f37(): Int {
        if (Build.VERSION.SDK_INT >= 23) { try { if (Build.VERSION.SDK_INT >= 22) println() } catch (e: Exception) {} }
        return 0
    }
    // CASE val-and
    fun f38(): Int {
        val a = Build.VERSION.SDK_INT >= 23 && Build.VERSION.SDK_INT >= 22
        return 0
    }
    // CASE val-or-lt23-ge22
    fun f39(): Int {
        val a = Build.VERSION.SDK_INT < 23 || Build.VERSION.SDK_INT >= 22
        return 0
    }
    // CASE val-or-lt23-ge23
    fun f40(): Int {
        val a = Build.VERSION.SDK_INT < 23 || Build.VERSION.SDK_INT >= 23
        return 0
    }
    // CASE val-or-ge23-ge22
    fun f41(): Int {
        val a = Build.VERSION.SDK_INT >= 23 || Build.VERSION.SDK_INT >= 22
        return 0
    }
    // CASE val-and-nest
    fun f42(): Int {
        val a = Build.VERSION.SDK_INT >= 23 && (Build.VERSION.SDK_INT >= 24 && Build.VERSION.SDK_INT >= 22)
        return 0
    }
    // CASE val-ifexpr-cond
    fun f43(): Int {
        val a = if (Build.VERSION.SDK_INT >= 23) Build.VERSION.SDK_INT >= 22 else false
        return 0
    }
    // CASE require
    fun f44(): Int {
        require(Build.VERSION.SDK_INT >= 23); if (Build.VERSION.SDK_INT >= 22) println()
        return 0
    }
    // CASE assert
    fun f45(): Int {
        assert(Build.VERSION.SDK_INT >= 23); if (Build.VERSION.SDK_INT >= 22) println()
        return 0
    }
}
class A1 {
    @RequiresApi(19) fun a() {}
    @RequiresApi(value = 19) fun b() {}
    @RequiresApi(api = 19) fun c() {}
    @RequiresApi(Build.VERSION_CODES.LOLLIPOP) fun d() {}
    @android.annotation.TargetApi(19) fun e() {}
    @TargetApi(value = 19) fun e2() {}
    @RequiresApi(19) val f = 1
    @RequiresApi(19) var g = 1
    @get:RequiresApi(19) val h = 1
    @field:RequiresApi(19) val i = 1
    @RequiresApi(19) constructor(a: Int)
    constructor(@RequiresApi(19) a: String)
    @RequiresApi(19) @TargetApi(19) fun j() {}
    @RequiresApi(19)
    @TargetApi(18)
    fun k() {}
    @RequiresApi(19, 20) fun l() {}
    @RequiresApi(22) fun m() {}
    @RequiresApi(Build.VERSION_CODES.M) fun n() {}
    @RequiresApi(MIN19) fun o() {}
    @RequiresApi(21) fun p() {}
    @RequiresApi(20 + 1) fun q() {}
    @RequiresApi(1) fun r() {}
    @RequiresApi(0) fun s() {}
    @RequiresApi(Build.VERSION_CODES.BASE) fun t() {}
    @RequiresApi(Build.VERSION_CODES.CUR_DEVELOPMENT) fun u() {}
    @TargetApi(Build.VERSION_CODES.CUR_DEVELOPMENT) fun v() {}
    @TargetApi("L") fun w() {}
    @TargetApi(19) fun w2() { if (Build.VERSION.SDK_INT >= 19) println() }
    @RequiresApi(19) object O {}
    @RequiresApi(19) companion object {}
    @RequiresApi(19) interface I
    @RequiresApi(19) enum class E { A }
    @RequiresApi(19) typealias TT = Int
}
const val MIN19 = 19
@RequiresApi(19) typealias TA = Int
@RequiresApi(19) fun topFun() {}
@RequiresApi(19) val topProp = 1
@file:RequiresApi(19)

