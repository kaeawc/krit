// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 12, 14
// kotlin.random.Random(seed) is as predictable as java.util.Random, and Go
// reports it once the file imports or mentions kotlin.random.Random.
package test

import javax.net.ssl.SSLContext
import kotlin.random.Random

fun longSeed(): Random = <!PrngFromSystemTime!>Random(System.nanoTime())<!>

fun intSeed(): Random = <!PrngFromSystemTime!>Random(System.currentTimeMillis().toInt())<!>

fun qualified(): Random = <!PrngFromSystemTime!>kotlin.random.Random(System.nanoTime())<!>

// Go misses this because it reads only the first unlabeled argument; the named
// seed is still the system clock.
fun namedSeed(): Random = <!PrngFromSystemTime!>Random(seed = System.nanoTime())<!>

fun fixedSeed(): Random = Random(42)

fun defaultInstance(): Int = Random.nextInt()

fun context(): SSLContext = SSLContext.getInstance("TLS")
