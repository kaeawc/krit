// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Properties whose values branch and refer to each other in a cycle, and
// properties that read a shared property more than once. Each property is
// walked once per key and its verdict reused, and a property read again while
// its own value is being walked counts as not hardcoded on that path. Without
// that, the walk re-enters the cycle on every branch and is exponential in
// its length: the cycles below never finish, bounded only by the depth.
package test

import java.util.Base64
import javax.crypto.spec.SecretKeySpec

val FLAG = System.getenv("FLAG") != null
val OTHER = System.getenv("OTHER") != null

// A cycle with no hardcoded branch holds no key.
val CYCLE_A: String get() = when { FLAG -> CYCLE_B; OTHER -> CYCLE_B; else -> CYCLE_B }
val CYCLE_B: String get() = when { FLAG -> CYCLE_A; OTHER -> CYCLE_A; else -> CYCLE_A }

// A cycle with a hardcoded branch holds the literal whenever that branch runs.
val LOOP_A: String get() = when { FLAG -> LOOP_B; OTHER -> LOOP_B; else -> "c2VjcmV0MTIzNDU2Nzg=" }
val LOOP_B: String get() = when { FLAG -> LOOP_A; OTHER -> LOOP_A; else -> LOOP_A }

// Each level reads the one below twice; every read is hardcoded.
val SHARED_0 = "c2Vj"
val SHARED_1 = SHARED_0 + SHARED_0
val SHARED_2 = SHARED_1 + SHARED_1
val SHARED_3 = SHARED_2 + SHARED_2
val SHARED_4 = SHARED_3 + SHARED_3
val SHARED_5 = SHARED_4 + SHARED_4
val SHARED_6 = SHARED_5 + SHARED_5
val SHARED_7 = SHARED_6 + SHARED_6
val SHARED_8 = SHARED_7 + SHARED_7
val SHARED_9 = SHARED_8 + SHARED_8
val SHARED_10 = SHARED_9 + SHARED_9
val SHARED_11 = SHARED_10 + SHARED_10
val SHARED_12 = SHARED_11 + SHARED_11
val SHARED_13 = SHARED_12 + SHARED_12
val SHARED_14 = SHARED_13 + SHARED_13
val SHARED_15 = SHARED_14 + SHARED_14
val SHARED_16 = SHARED_15 + SHARED_15
val SHARED_17 = SHARED_16 + SHARED_16
val SHARED_18 = SHARED_17 + SHARED_17
val SHARED_19 = SHARED_18 + SHARED_18
val SHARED_20 = SHARED_19 + SHARED_19
val SHARED_21 = SHARED_20 + SHARED_20
val SHARED_22 = SHARED_21 + SHARED_21
val SHARED_23 = SHARED_22 + SHARED_22
val SHARED_24 = SHARED_23 + SHARED_23

class Crypto {
    fun cycles() {
        SecretKeySpec(Base64.getDecoder().decode(CYCLE_A), "AES")
        SecretKeySpec(Base64.getDecoder().decode(CYCLE_B), "AES")
    }

    // Go misses these because the key argument holds no quote; FIR is correct
    // because each value is the literal on at least one path.
    fun loops() {
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(LOOP_A), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(LOOP_B), "AES")
    }

    // Go misses this because the key argument holds no quote; FIR is correct
    // because every read resolves to the same literal.
    fun shared() {
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(SHARED_24), "AES")
    }
}
