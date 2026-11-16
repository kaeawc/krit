package p

import android.annotation.TargetApi
import android.app.Activity
import android.os.Build
import android.widget.TextView
import androidx.annotation.RequiresApi

class K1 : Activity() {
    fun a() {
        if (Build.VERSION.SDK_INT >= 19) { println("a") }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.KITKAT) { println("b") }
        if (Build.VERSION.SDK_INT >= 21) { println("c") }
        if (Build.VERSION.SDK_INT >= 23) { println("d") }
        if (Build.VERSION.SDK_INT < 21) { println("e") }
        if (Build.VERSION.SDK_INT < 19) { println("f") }
        if (Build.VERSION.SDK_INT > 19) { println("g") }
        if (Build.VERSION.SDK_INT > 20) { println("g2") }
        if (Build.VERSION.SDK_INT <= 20) { println("h") }
        if (Build.VERSION.SDK_INT <= 21) { println("h2") }
        if (Build.VERSION.SDK_INT == 19) { println("i") }
        if (Build.VERSION.SDK_INT != 19) { println("j") }
        if (19 <= Build.VERSION.SDK_INT) { println("k") }
        if (Build.VERSION.SDK_INT >= 19 && x()) { println("l") }
        if (Build.VERSION.SDK_INT >= 19 || x()) { println("m") }
        val z = Build.VERSION.SDK_INT >= 18
        val y = Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP
        val w = if (Build.VERSION.SDK_INT >= 17) 1 else 2
        when {
            Build.VERSION.SDK_INT >= 19 -> println("w")
            else -> println("x")
        }
        if (Build.VERSION.SDK_INT >= 22) { println("n") } else { println("o") }
        if (Build.VERSION.SDK_INT in 15..30) { println("p") }
        if (Build.VERSION.SDK_INT >= 100) { println("q") }
        if (Build.VERSION.SDK_INT >= 21 && Build.VERSION.SDK_INT < 19) { println("r") }
    }
    private fun x() = true

    @TargetApi(19)
    fun t1() {}
    @TargetApi(21)
    fun t2() {}
    @TargetApi(Build.VERSION_CODES.KITKAT)
    fun t3() {}
    @RequiresApi(19)
    fun t4() {}
    @RequiresApi(api = 18)
    fun t5() {}
    @RequiresApi(Build.VERSION_CODES.JELLY_BEAN)
    fun t6() {}
    @RequiresApi(22)
    fun t7() {}
    @androidx.annotation.RequiresApi(20)
    fun t8() {}
    @TargetApi(19)
    class Inner
    fun cast() {
        val t = findViewById<TextView>(1) as TextView
        val u = findViewById(2) as TextView
        val v: TextView = findViewById(3)
        val w = findViewById<TextView>(4)
    }
}

fun lam(f: () -> Unit, x: Int) {}
fun lam2(x: Int, f: () -> Unit) {}
fun lam3(f: () -> Unit, g: () -> Unit) {}
fun lam4(f: (Int) -> Unit, x: Int = 3) {}
fun lam5(f: (Int) -> Unit, vararg x: Int) {}
private fun lam6(f: () -> Unit, x: Int) {}
class KC {
    fun m(f: () -> Unit, x: Int) {}
    internal fun m2(f: () -> Unit, x: Int) {}
    constructor(f: () -> Unit, x: Int)
}
fun lam7(f: suspend () -> Unit, x: Int) {}
fun lam8(f: Function0<Unit>, x: Int) {}
fun lam9(x: Int, f: () -> Unit, y: Int) {}
inline fun lam10(f: () -> Unit, x: Int) {}
fun TextView.lam11(f: () -> Unit, x: Int) {}
