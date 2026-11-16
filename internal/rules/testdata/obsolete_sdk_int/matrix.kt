package p

import android.annotation.TargetApi
import android.os.Build
import android.os.Build.VERSION.SDK_INT
import androidx.annotation.RequiresApi

const val MINV = 19
const val NEWV = 23

class K2 {
    fun x() = true
    // CASE >= 20
    fun f1(): Int { if (Build.VERSION.SDK_INT >= 20) println(); return 0 }
    // CASE >= 21
    fun f2(): Int { if (Build.VERSION.SDK_INT >= 21) println(); return 0 }
    // CASE >= 22
    fun f3(): Int { if (Build.VERSION.SDK_INT >= 22) println(); return 0 }
    // CASE >= 23
    fun f4(): Int { if (Build.VERSION.SDK_INT >= 23) println(); return 0 }
    // CASE > 20
    fun f5(): Int { if (Build.VERSION.SDK_INT > 20) println(); return 0 }
    // CASE > 21
    fun f6(): Int { if (Build.VERSION.SDK_INT > 21) println(); return 0 }
    // CASE > 22
    fun f7(): Int { if (Build.VERSION.SDK_INT > 22) println(); return 0 }
    // CASE > 23
    fun f8(): Int { if (Build.VERSION.SDK_INT > 23) println(); return 0 }
    // CASE < 20
    fun f9(): Int { if (Build.VERSION.SDK_INT < 20) println(); return 0 }
    // CASE < 21
    fun f10(): Int { if (Build.VERSION.SDK_INT < 21) println(); return 0 }
    // CASE < 22
    fun f11(): Int { if (Build.VERSION.SDK_INT < 22) println(); return 0 }
    // CASE < 23
    fun f12(): Int { if (Build.VERSION.SDK_INT < 23) println(); return 0 }
    // CASE <= 20
    fun f13(): Int { if (Build.VERSION.SDK_INT <= 20) println(); return 0 }
    // CASE <= 21
    fun f14(): Int { if (Build.VERSION.SDK_INT <= 21) println(); return 0 }
    // CASE <= 22
    fun f15(): Int { if (Build.VERSION.SDK_INT <= 22) println(); return 0 }
    // CASE <= 23
    fun f16(): Int { if (Build.VERSION.SDK_INT <= 23) println(); return 0 }
    // CASE == 20
    fun f17(): Int { if (Build.VERSION.SDK_INT == 20) println(); return 0 }
    // CASE == 21
    fun f18(): Int { if (Build.VERSION.SDK_INT == 21) println(); return 0 }
    // CASE == 22
    fun f19(): Int { if (Build.VERSION.SDK_INT == 22) println(); return 0 }
    // CASE == 23
    fun f20(): Int { if (Build.VERSION.SDK_INT == 23) println(); return 0 }
    // CASE != 20
    fun f21(): Int { if (Build.VERSION.SDK_INT != 20) println(); return 0 }
    // CASE != 21
    fun f22(): Int { if (Build.VERSION.SDK_INT != 21) println(); return 0 }
    // CASE != 22
    fun f23(): Int { if (Build.VERSION.SDK_INT != 22) println(); return 0 }
    // CASE != 23
    fun f24(): Int { if (Build.VERSION.SDK_INT != 23) println(); return 0 }
    // CASE bare
    fun f25(): Int { if (SDK_INT >= 19) println(); return 0 }
    // CASE fq
    fun f26(): Int { if (android.os.Build.VERSION.SDK_INT >= 19) println(); return 0 }
    // CASE paren
    fun f27(): Int { if ((Build.VERSION.SDK_INT >= 19)) println(); return 0 }
    // CASE not-ge19
    fun f28(): Int { if (!(Build.VERSION.SDK_INT >= 19)) println(); return 0 }
    // CASE not-ge23
    fun f29(): Int { if (!(Build.VERSION.SDK_INT >= 23)) println(); return 0 }
    // CASE constval19
    fun f30(): Int { if (Build.VERSION.SDK_INT >= MINV) println(); return 0 }
    // CASE constval23
    fun f31(): Int { if (Build.VERSION.SDK_INT >= NEWV) println(); return 0 }
    // CASE localvar
    fun f32(): Int { val v = Build.VERSION.SDK_INT; if (v >= 19) println(); return 0 }
    // CASE codes-kitkat
    fun f33(): Int { if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.KITKAT) println(); return 0 }
    // CASE codes-M
    fun f34(): Int { if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) println(); return 0 }
    // CASE sum
    fun f35(): Int { if (Build.VERSION.SDK_INT >= 20 + 1) println(); return 0 }
    // CASE sum19
    fun f36(): Int { if (Build.VERSION.SDK_INT >= 15 + 5) println(); return 0 }
    // CASE in10_20
    fun f37(): Int { if (Build.VERSION.SDK_INT in 10..20) println(); return 0 }
    // CASE notin10_20
    fun f38(): Int { if (Build.VERSION.SDK_INT !in 10..20) println(); return 0 }
    // CASE nest-ge23-ge22
    fun f39(): Int { if (Build.VERSION.SDK_INT >= 23) { if (Build.VERSION.SDK_INT >= 22) println() }; return 0 }
    // CASE nest-ge23-ge23
    fun f40(): Int { if (Build.VERSION.SDK_INT >= 23) { if (Build.VERSION.SDK_INT >= 23) println() }; return 0 }
    // CASE nest-ge23-ge24
    fun f41(): Int { if (Build.VERSION.SDK_INT >= 23) { if (Build.VERSION.SDK_INT >= 24) println() }; return 0 }
    // CASE nest-ge23-lt22
    fun f42(): Int { if (Build.VERSION.SDK_INT >= 23) { if (Build.VERSION.SDK_INT < 22) println() }; return 0 }
    // CASE nest-ge23-lt23
    fun f43(): Int { if (Build.VERSION.SDK_INT >= 23) { if (Build.VERSION.SDK_INT < 23) println() }; return 0 }
    // CASE nest-lt23-ge23
    fun f44(): Int { if (Build.VERSION.SDK_INT < 23) { if (Build.VERSION.SDK_INT >= 23) println() }; return 0 }
    // CASE nest-lt23-ge22
    fun f45(): Int { if (Build.VERSION.SDK_INT < 23) { if (Build.VERSION.SDK_INT >= 22) println() }; return 0 }
    // CASE nest-lt23-ge21
    fun f46(): Int { if (Build.VERSION.SDK_INT < 23) { if (Build.VERSION.SDK_INT >= 21) println() }; return 0 }
    // CASE else-lt23-ge23
    fun f47(): Int { if (Build.VERSION.SDK_INT < 23) { println() } else { if (Build.VERSION.SDK_INT >= 23) println() }; return 0 }
    // CASE else-lt23-ge22
    fun f48(): Int { if (Build.VERSION.SDK_INT < 23) { println() } else { if (Build.VERSION.SDK_INT >= 22) println() }; return 0 }
    // CASE else-ge23-lt23
    fun f49(): Int { if (Build.VERSION.SDK_INT >= 23) { println() } else { if (Build.VERSION.SDK_INT < 23) println() }; return 0 }
    // CASE else-ge23-lt22
    fun f50(): Int { if (Build.VERSION.SDK_INT >= 23) { println() } else { if (Build.VERSION.SDK_INT < 22) println() }; return 0 }
    // CASE else-ge23-lt21
    fun f51(): Int { if (Build.VERSION.SDK_INT >= 23) { println() } else { if (Build.VERSION.SDK_INT < 21) println() }; return 0 }
    // CASE early-lt23-ge23
    fun f52(): Int { if (Build.VERSION.SDK_INT < 23) return 1; if (Build.VERSION.SDK_INT >= 23) println(); return 0 }
    // CASE early-lt23-ge22
    fun f53(): Int { if (Build.VERSION.SDK_INT < 23) return 1; if (Build.VERSION.SDK_INT >= 22) println(); return 0 }
    // CASE early-lt23-lt24
    fun f54(): Int { if (Build.VERSION.SDK_INT < 23) return 1; if (Build.VERSION.SDK_INT < 24) println(); return 0 }
    // CASE and-ge23-ge22
    fun f55(): Int { if (Build.VERSION.SDK_INT >= 23 && Build.VERSION.SDK_INT >= 22) println(); return 0 }
    // CASE and-ge22-ge23
    fun f56(): Int { if (Build.VERSION.SDK_INT >= 22 && Build.VERSION.SDK_INT >= 23) println(); return 0 }
    // CASE or-ge23-ge22
    fun f57(): Int { if (Build.VERSION.SDK_INT >= 23 || Build.VERSION.SDK_INT >= 22) println(); return 0 }
    // CASE x-and-ge19
    fun f58(): Int { if (x() && Build.VERSION.SDK_INT >= 19) println(); return 0 }
    // CASE x-or-ge19
    fun f59(): Int { if (x() || Build.VERSION.SDK_INT >= 19) println(); return 0 }
    // CASE x-and-ge23-ge22
    fun f60(): Int { if (x() && Build.VERSION.SDK_INT >= 23 && Build.VERSION.SDK_INT >= 22) println(); return 0 }
    // CASE while
    fun f61(): Int { while (Build.VERSION.SDK_INT >= 19) { break }; return 0 }
    // CASE valbool
    fun f62(): Int { val b = x() && Build.VERSION.SDK_INT >= 19; return 0 }
    // CASE arg
    fun f63(): Int { println(Build.VERSION.SDK_INT >= 19); return 0 }
    // CASE valbool2
    fun f64(): Int { val r = Build.VERSION.SDK_INT >= 19 && x(); return 0 }
    // CASE when
    fun f65(): Int { val s = when { Build.VERSION.SDK_INT >= 19 -> 1; else -> 2 }; return 0 }
    // CASE when23-22
    fun f66(): Int { val s = when { Build.VERSION.SDK_INT >= 23 -> 1; Build.VERSION.SDK_INT >= 22 -> 2; else -> 3 }; return 0 }
    // CASE ifexpr23
    fun f67(): Int { val s = if (Build.VERSION.SDK_INT >= 23) 1 else 2; return 0 }
    // CASE ifexpr19
    fun f68(): Int { val s = if (Build.VERSION.SDK_INT >= 19) 1 else 2; return 0 }
    // CASE lambda
    fun f69(): Int { val g: () -> Boolean = { Build.VERSION.SDK_INT >= 19 }; return 0 }
    // CASE runlambda
    fun f70(): Int { run { if (Build.VERSION.SDK_INT >= 19) println() }; return 0 }
    // CASE eqtrue
    fun f71(): Int { if (Build.VERSION.SDK_INT >= 19 == true) println(); return 0 }
    // CASE rev-19-le
    fun f72(): Int { if (19 <= Build.VERSION.SDK_INT) println(); return 0 }
    // CASE rev-22-gt
    fun f73(): Int { if (22 > Build.VERSION.SDK_INT) println(); return 0 }
    // CASE rev-21-gt
    fun f74(): Int { if (21 > Build.VERSION.SDK_INT) println(); return 0 }
    // CASE rev-20-ge
    fun f75(): Int { if (20 >= Build.VERSION.SDK_INT) println(); return 0 }
    // CASE rev-23le-lt24
    fun f76(): Int { if (23 <= Build.VERSION.SDK_INT && Build.VERSION.SDK_INT < 24) println(); return 0 }
    @RequiresApi(23)
    fun r23(): Int {
        // CASE ra23-ge22
        if (Build.VERSION.SDK_INT >= 22) println()
        // CASE ra23-ge23
        if (Build.VERSION.SDK_INT >= 23) println()
        // CASE ra23-ge24
        if (Build.VERSION.SDK_INT >= 24) println()
        // CASE ra23-lt23
        if (Build.VERSION.SDK_INT < 23) println()
        // CASE ra23-lt22
        if (Build.VERSION.SDK_INT < 22) println()
        // CASE ra23-lt24
        if (Build.VERSION.SDK_INT < 24) println()
        return 0
    }
    @TargetApi(23)
    fun t23(): Int {
        // CASE ta23-ge23
        if (Build.VERSION.SDK_INT >= 23) println()
        // CASE ta23-ge22
        if (Build.VERSION.SDK_INT >= 22) println()
        // CASE ta23-lt23
        if (Build.VERSION.SDK_INT < 23) println()
        return 0
    }
    @RequiresApi(23)
    val p23: Int get() {
        // CASE ra23-prop-ge23
        if (Build.VERSION.SDK_INT >= 23) println()
        return 0
    }
    fun lam() {
        @RequiresApi(23)
        fun inner() {
            // CASE ra23-local-ge23
            if (Build.VERSION.SDK_INT >= 23) println()
        }
    }
}
@RequiresApi(23)
class K3 {
    fun a() {
        // CASE ra23-class-ge23
        if (Build.VERSION.SDK_INT >= 23) println()
        // CASE ra23-class-ge22
        if (Build.VERSION.SDK_INT >= 22) println()
    }
}
@RequiresApi(23)
fun top23() {
    // CASE ra23-top-ge23
    if (Build.VERSION.SDK_INT >= 23) println()
}
val top = if (Build.VERSION.SDK_INT >= 19) 1 else 2
class K4 { val f = Build.VERSION.SDK_INT >= 19
  init { if (Build.VERSION.SDK_INT >= 19) println() }
  constructor(a: Int) { if (Build.VERSION.SDK_INT >= 19) println() }
}
