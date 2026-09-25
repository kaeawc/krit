// RENDER_DIAGNOSTICS_FULL_TEXT
// Deliberate improvements: each IV below is built from inline literal bytes,
// so each call is a real finding. Go misses them because it parses the
// argument's source text: its number parser rejects `-0x10`, `0b1`, `(1)`, a
// comment, and a trailing comma inside byteArrayOf; it reads an annotated
// argument as the annotation; it accepts no call chained on byteArrayOf; and
// it matches only `Base64.decode(` and `Base64.getDecoder().decode(`. FIR
// reads the resolved arguments instead.
package test

import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.IvParameterSpec
import kotlin.io.encoding.Base64

class Crypto {
    fun negativeHex() = <!StaticIv!>IvParameterSpec(byteArrayOf(-0x10, 1))<!>

    fun binary() = <!StaticIv!>IvParameterSpec(byteArrayOf(0b1, 0B10))<!>

    fun parenthesized() = <!StaticIv!>IvParameterSpec(byteArrayOf((1), (-2)))<!>

    fun comment() = <!StaticIv!>IvParameterSpec(byteArrayOf(1, /* two */ 2))<!>

    fun trailingComma() = <!StaticIv!>IvParameterSpec(byteArrayOf(1, 2,))<!>

    fun annotated() = <!StaticIv!>IvParameterSpec(@Suppress("x") byteArrayOf(1, 2))<!>

    fun copied() = <!StaticIv!>GCMParameterSpec(128, byteArrayOf(1, 2).copyOf(12))<!>

    fun defaultDecoder() = <!StaticIv!>IvParameterSpec(Base64.Default.decode("AAAAAAAAAAAAAAAAAAAAAA=="))<!>

    fun urlSafeDecoder() = <!StaticIv!>IvParameterSpec(Base64.UrlSafe.decode("AAAAAAAAAAAAAAAAAAAAAA=="))<!>

    fun urlDecoder() = <!StaticIv!>IvParameterSpec(java.util.Base64.getUrlDecoder().decode("AAAAAAAAAAAAAAAAAAAAAA=="))<!>

    fun heldDecoder(): IvParameterSpec {
        val decoder = java.util.Base64.getDecoder()
        return <!StaticIv!>IvParameterSpec(decoder.decode("AAAAAAAAAAAAAAAAAAAAAA=="))<!>
    }
}
