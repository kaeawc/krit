// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10
// Star imports: `import java.util.*` brings in java.util.Random and
// `import java.security.*` is a security import, for Go and FIR alike.
package test

import java.security.*
import java.util.*

fun seeded(): Random = <!PrngFromSystemTime!>Random(System.nanoTime())<!>

fun digest(): MessageDigest = MessageDigest.getInstance("SHA-256")
