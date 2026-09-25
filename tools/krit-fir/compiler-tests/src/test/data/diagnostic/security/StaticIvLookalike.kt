// RENDER_DIAGNOSTICS_FULL_TEXT
// Deliberate precision difference from the Go rule: a same-package
// `String.toByteArray()` wins over the kotlin.text default import, and this one
// returns random bytes. Go reports the call because it matches the method name
// in the argument's text; FIR reports only kotlin.text.toByteArray.
package test

import java.security.SecureRandom
import javax.crypto.spec.IvParameterSpec

fun String.toByteArray(): ByteArray = ByteArray(length).also { SecureRandom().nextBytes(it) }

class Crypto {
    fun lookalike() = IvParameterSpec("0123456789abcdef".toByteArray())

    fun stdlib() = <!StaticIv!>IvParameterSpec("0123456789abcdef".toByteArray(Charsets.UTF_8))<!>
}
