// RENDER_DIAGNOSTICS_FULL_TEXT
// Divergence (recall): each call below constructs java.security.SecureRandom
// with a seed, and FIR reports it. Go misses all of them: it needs the literal
// spelling `SecureRandom` with an explicit java.security.SecureRandom import
// and no kotlin.random.Random import, or the literal java.security.SecureRandom
// qualifier. An import alias, a typealias, and a backticked name are other
// spellings, and this file also imports kotlin.random.Random.
package test

import java.security.SecureRandom
import java.security.SecureRandom as Csprng
import kotlin.random.Random

typealias SeededSource = java.security.SecureRandom

fun importAlias(): SecureRandom = <!TrulyRandom!>Csprng(byteArrayOf(1))<!>

fun typeAlias(): SecureRandom = <!TrulyRandom!>SeededSource(byteArrayOf(1))<!>

fun backticked(): SecureRandom = <!TrulyRandom!>`SecureRandom`(byteArrayOf(1))<!>

fun qualifiedBackticked(): SecureRandom = <!TrulyRandom!>java.security.`SecureRandom`(byteArrayOf(1))<!>

// Go skips a bare `SecureRandom(seed)` whenever the file imports
// kotlin.random.Random, but the call still constructs java.security.SecureRandom.
fun alongsideKotlinRandom(): Int = <!TrulyRandom!>SecureRandom(byteArrayOf(1))<!>.nextInt() + Random.nextInt()

fun aliasDefault(): SecureRandom = Csprng()

fun typeAliasDefault(): SecureRandom = SeededSource()
