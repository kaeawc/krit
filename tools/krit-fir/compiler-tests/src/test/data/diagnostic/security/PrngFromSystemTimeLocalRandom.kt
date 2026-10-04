// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12
// Go reports this call because it is spelled Random and this comment mentions
// java.util.Random. The call builds the local Random class below, not a
// java.util.Random, so FIR reports nothing.
package test

import javax.crypto.Cipher

class Random(val seed: Long)

fun local(): Random = Random(System.nanoTime())

fun cipher(): Cipher = Cipher.getInstance("AES/GCM/NoPadding")
