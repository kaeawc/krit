// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: literal sources behind null-safety and cast wrappers, a String
// transform, a harmless lambda, or a template of literal-initialized vals. Go
// reports every call here: the argument text starts with a string literal and
// `.toByteArray(`, or holds `Base64.getDecoder().decode(` and a quote. Each IV
// is built from literal bytes, so FIR reports them too.
package test

import android.util.Log
import java.util.Base64
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.IvParameterSpec

val PREFIX = "0123456789"
val NESTED = "$PREFIX-ab"

class Crypto {
    fun safeCall() = <!StaticIv!>IvParameterSpec(Base64.getDecoder().decode("AAAAAAAAAAAAAAAAAAAAAA==")?.copyOf(16))<!>

    fun notNull() = <!StaticIv!>IvParameterSpec(Base64.getDecoder().decode("AAAAAAAAAAAAAAAAAAAAAA==")!!)<!>

    fun elvis() = <!StaticIv!>GCMParameterSpec(128, Base64.getDecoder().decode("AAAAAAAAAAAAAAAA") ?: ByteArray(12))<!>

    fun stringSafeCall() = <!StaticIv!>IvParameterSpec("0123456789abcdef".toByteArray()?.copyOf(16))<!>

    fun cast() = <!StaticIv!>IvParameterSpec("0123456789abcdef".toByteArray() as ByteArray)<!>

    fun safeCast() = <!StaticIv!>IvParameterSpec("0123456789abcdef".toByteArray() as? ByteArray)<!>

    fun trimmed() = <!StaticIv!>IvParameterSpec<!>(
        Base64.getDecoder().decode(
            """
            AAAAAAAAAAAA
            AAAAAAAAAA==
            """.trimIndent()
        )
    )

    fun joined() = <!StaticIv!>IvParameterSpec(Base64.getDecoder().decode("AAAA AAAA AAAA AAAA AAAA AA==".replace(" ", "")))<!>

    fun regex() = <!StaticIv!>IvParameterSpec(Base64.getDecoder().decode("AAAA AAAA".replace(Regex("\\s"), "").padEnd(24, 'A')))<!>

    fun logged() = <!StaticIv!>IvParameterSpec("0123456789abcdef".toByteArray().also { Log.d("iv", "built") })<!>

    fun letCopy() = <!StaticIv!>IvParameterSpec("0123456789".toByteArray().let { it.copyOf(16) })<!>

    fun runCopy() = <!StaticIv!>IvParameterSpec("0123456789".toByteArray().run { copyOf(16) })<!>

    // Setting a byte to a literal keeps the IV static.
    fun literalSet() = <!StaticIv!>IvParameterSpec("0123456789abcdef".toByteArray().apply { this[0] = 1 })<!>

    fun reference() = <!StaticIv!>IvParameterSpec("0123456789abcdef".toByteArray().let(::identity))<!>

    fun valTemplate() = <!StaticIv!>IvParameterSpec("$PREFIX-abcde".toByteArray())<!>

    fun nestedValTemplate() = <!StaticIv!>IvParameterSpec("$NESTED-abc".toByteArray())<!>

    fun localValTemplate(): IvParameterSpec {
        val prefix = "0123456789"
        return <!StaticIv!>IvParameterSpec("$prefix-abcde".toByteArray())<!>
    }
}

fun identity(bytes: ByteArray): ByteArray = bytes
