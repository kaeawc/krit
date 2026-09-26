// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): each marked call constructs java.util.Random, or seeds
// a SecureRandom with a literal, and FIR reports it. Go misses all of them: it
// needs the spelling `Random` / `SecureRandom` with an explicit import and no
// kotlin.random.Random import in the file, or the literal package qualifier.
// An import alias is another spelling, and this file also imports
// kotlin.random.Random.
package test

import java.security.SecureRandom as Csprng
import java.util.Random as JavaRandom
import kotlin.random.Random

fun importAlias(): JavaRandom = <!SecureRandom!>JavaRandom()<!>

fun aliasedSecureRandom() {
    <!SecureRandom!>Csprng().setSeed(1L)<!>
}

fun kotlinRandom(): Int = Random.nextInt()
