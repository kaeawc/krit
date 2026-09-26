// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19
// The seed is the argument bound to the first parameter (Go: the first
// unlabeled argument). A clock read passed to any other parameter does not
// seed the Random, so neither Go nor FIR reports it.
package test

import javax.crypto.Cipher

class Random(seed: Long, val createdAt: Long) : java.util.Random(seed)

// The seed is 42; the clock read is createdAt.
fun stamped(): Random = Random(42L, System.currentTimeMillis())

// The seed is 42, named; Go reads only unlabeled arguments.
fun namedStamped(): Random = Random(createdAt = System.nanoTime(), seed = 42L)

// The seed is the first argument, the system clock.
fun clockSeeded(): Random = <!PrngFromSystemTime!>Random(System.nanoTime(), 42L)<!>

fun cipher(): Cipher = Cipher.getInstance("AES/GCM/NoPadding")
