package p

import android.os.Build
import android.os.Build.VERSION
import android.os.Build.VERSION.SDK_INT
import android.os.Build.VERSION_CODES
import android.os.Build.VERSION_CODES.KITKAT
import android.os.Build.VERSION_CODES.M as MARSH

object Consts {
    const val C19 = 19
    const val C23 = 23
}
class H {
    companion object {
        const val CC19 = 19
    }
}
const val T19 = 19
const val TSUM = T19 + 1
const val TALIAS = T19
val nonConst = 19

class K6 {
    private val priv = 19
    private fun x() = true
    fun a() {
        if (SDK_INT in 10..20) println("r1")
        if (SDK_INT in 10..30) println("r2")
        if (SDK_INT in 22..30) println("r3")
        if (SDK_INT in 10 until 20) println("r4")
        if (SDK_INT in 10..<20) println("r5")
        if (SDK_INT !in 22..30) println("r6")
        if (SDK_INT !in 10..30) println("r7")
        if (SDK_INT in 10..20 && x()) println("r8")
        if (VERSION.SDK_INT >= 19) println("n1")
        if (VERSION.SDK_INT >= VERSION_CODES.KITKAT) println("n2")
        if (SDK_INT >= KITKAT) println("n3")
        if (SDK_INT >= MARSH) println("n4")
        if (SDK_INT >= Consts.C19) println("n5")
        if (SDK_INT >= H.CC19) println("n6")
        if (SDK_INT >= T19) println("n7")
        if (SDK_INT >= TSUM) println("n8")
        if (SDK_INT >= TALIAS) println("n9")
        if (SDK_INT >= nonConst) println("n10")
        if (SDK_INT >= priv) println("n11")
        val loc = 19
        if (SDK_INT >= loc) println("n12")
        if (SDK_INT >= (19)) println("n13")
        if (SDK_INT >= 0x13) println("n14")
        if (SDK_INT >= 19L) println("n15")
        if (SDK_INT.toLong() >= 19) println("n16")
        if (SDK_INT >= -1) println("n17")
        if (SDK_INT >= 10_000) println("n18")
        if (SDK_INT >= Build.VERSION_CODES.CUR_DEVELOPMENT) println("n19")
        if (SDK_INT >= Build.VERSION_CODES.BASE) println("n20")
        if (SDK_INT > 0) println("n21")
        if (SDK_INT >= 1) println("n23")
        if (SDK_INT < 100) println("n24")
        if (SDK_INT <= 100) println("n25")
        if (SDK_INT > 100) println("n26")
        if (SDK_INT >= SDK_INT) println("n27")
        if (SDK_INT >= 19 && SDK_INT < 100) println("n28")
        if (SDK_INT == Build.VERSION_CODES.LOLLIPOP) println("n29")
        if (SDK_INT >= 21.toInt()) println("n30")
        if (Build.VERSION.SDK_INT.compareTo(19) >= 0) println("n31")
        if (SDK_INT >= 19 != x()) println("n32")
        if ((SDK_INT >= 19) == x()) println("n33")
        val v1 = SDK_INT >= 19
        if (v1) println("n34")
        if (SDK_INT >= 19 && SDK_INT >= 20 && SDK_INT >= 21) println("n35")
        val arr = intArrayOf(1, 2)
        if (arr[0] >= 19) println("n36")
        if (Build.VERSION.PREVIEW_SDK_INT >= 19) println("n37")
        if (VERSION.SDK_INT >= 19) println("n39")
        val t = 19
        println(SDK_INT >= t)
    }
}
