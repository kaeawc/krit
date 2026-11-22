// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// This constant message checks that repeated references share each property's classification result.
package test.logconditional.performance

import android.util.Log

fun repeatedLocalValues() {
    val n0 = "x"
    val n1 = n0 + n0
    val n2 = n1 + n1
    val n3 = n2 + n2
    val n4 = n3 + n3
    val n5 = n4 + n4
    val n6 = n5 + n5
    val n7 = n6 + n6
    val n8 = n7 + n7
    val n9 = n8 + n8
    val n10 = n9 + n9
    val n11 = n10 + n10
    val n12 = n11 + n11
    val n13 = n12 + n12
    val n14 = n13 + n13
    val n15 = n14 + n14
    val n16 = n15 + n15
    val n17 = n16 + n16
    val n18 = n17 + n17
    val n19 = n18 + n18
    val n20 = n19 + n19
    val n21 = n20 + n20
    val n22 = n21 + n21
    val n23 = n22 + n22
    val n24 = n23 + n23
    val n25 = n24 + n24
    val n26 = n25 + n25
    Log.d("Tag", "n=" + n26)
}
