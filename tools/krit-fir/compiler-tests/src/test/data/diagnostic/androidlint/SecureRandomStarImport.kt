// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): a java.util star import resolves the bare Random name
// to java.util.Random, and a java.security star import resolves SecureRandom,
// so both calls report. Go misses them because it needs explicit
// java.util.Random / java.security.SecureRandom imports.
package test

import java.security.*
import java.util.*

fun unseeded(): Random = <!SecureRandom!>Random()<!>

fun seeded() {
    <!SecureRandom!>SecureRandom().setSeed(1L)<!>
}
