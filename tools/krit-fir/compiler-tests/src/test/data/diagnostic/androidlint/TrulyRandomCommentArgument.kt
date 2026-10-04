// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 12, 16
// Divergence (precision): an argument list that holds only a comment passes no
// seed; it calls the no-argument constructor. Go counts the comment node as an
// argument and reports it; FIR does not.
package test

import java.security.SecureRandom

fun commentOnly(): SecureRandom = SecureRandom(/* no seed */)

fun lineComment(): SecureRandom = SecureRandom(
    // self-seeded
)

fun commentAndSeed(): SecureRandom = <!TrulyRandom!>SecureRandom(/* fixed */ byteArrayOf(1))<!>
