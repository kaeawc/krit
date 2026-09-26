// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 20, 24
// Factory functions spelled Random: Go reports any call spelled `Random(...)`
// once the file mentions java.util.Random. A factory that returns a
// java.util.Random or kotlin.random.Random built from its first argument
// makes the same predictable generator, so FIR reports it too. A function
// named Random that returns neither creates no Random at all.
package test

import javax.crypto.Cipher

fun Random(seed: Long, tag: String): java.util.Random = java.util.Random(seed)

fun Random(seed: Long, fast: Boolean): kotlin.random.Random = kotlin.random.Random(seed)

fun Random(seed: Long, label: Char): String = "$label$seed"

fun javaFactory(): java.util.Random = <!PrngFromSystemTime!>Random(System.nanoTime(), "x")<!>

fun kotlinFactory(): kotlin.random.Random = <!PrngFromSystemTime!>Random(System.currentTimeMillis(), true)<!>

// Go reports this because the call is spelled Random. The function returns a
// String, so no java.util.Random is seeded.
fun label(): String = Random(System.nanoTime(), 'x')

fun cipher(): Cipher = Cipher.getInstance("AES/GCM/NoPadding")
