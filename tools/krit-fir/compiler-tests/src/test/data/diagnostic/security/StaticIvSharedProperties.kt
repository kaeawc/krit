// RENDER_DIAGNOSTICS_FULL_TEXT
// A template over properties that read a shared property more than once.
// Each property is walked once per check and its verdict reused. Without
// that, the walk is exponential in the chain length: the 40-level chain below
// never finishes. (A cycle of initializers does not compile: K2 reports that
// the later property must be initialized.)
package test

import javax.crypto.spec.IvParameterSpec

val IV_0 = "0"
val IV_1 = "$IV_0$IV_0"
val IV_2 = "$IV_1$IV_1"
val IV_3 = "$IV_2$IV_2"
val IV_4 = "$IV_3$IV_3"
val IV_5 = "$IV_4$IV_4"
val IV_6 = "$IV_5$IV_5"
val IV_7 = "$IV_6$IV_6"
val IV_8 = "$IV_7$IV_7"
val IV_9 = "$IV_8$IV_8"
val IV_10 = "$IV_9$IV_9"
val IV_11 = "$IV_10$IV_10"
val IV_12 = "$IV_11$IV_11"
val IV_13 = "$IV_12$IV_12"
val IV_14 = "$IV_13$IV_13"
val IV_15 = "$IV_14$IV_14"
val IV_16 = "$IV_15$IV_15"
val IV_17 = "$IV_16$IV_16"
val IV_18 = "$IV_17$IV_17"
val IV_19 = "$IV_18$IV_18"
val IV_20 = "$IV_19$IV_19"
val IV_21 = "$IV_20$IV_20"
val IV_22 = "$IV_21$IV_21"
val IV_23 = "$IV_22$IV_22"
val IV_24 = "$IV_23$IV_23"
val IV_25 = "$IV_24$IV_24"
val IV_26 = "$IV_25$IV_25"
val IV_27 = "$IV_26$IV_26"
val IV_28 = "$IV_27$IV_27"
val IV_29 = "$IV_28$IV_28"
val IV_30 = "$IV_29$IV_29"
val IV_31 = "$IV_30$IV_30"
val IV_32 = "$IV_31$IV_31"
val IV_33 = "$IV_32$IV_32"
val IV_34 = "$IV_33$IV_33"
val IV_35 = "$IV_34$IV_34"
val IV_36 = "$IV_35$IV_35"
val IV_37 = "$IV_36$IV_36"
val IV_38 = "$IV_37$IV_37"
val IV_39 = "$IV_38$IV_38"
val IV_40 = "$IV_39$IV_39"

class Crypto {
    // Go reports this: the argument starts with a string literal followed by
    // `.toByteArray(`. Every entry resolves to the literal "0".
    fun shared() = <!StaticIv!>IvParameterSpec("$IV_40".toByteArray())<!>
}
