// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Deliberate improvements: each IV below is built from inline literal bytes,
// so each call is a real finding. Go misses them because it parses the
// argument's source text: its number parser rejects `-0x10`, `0b1`, `(1)`, a
// comment, and a trailing comma inside byteArrayOf; it reads an annotated
// argument as the annotation; it accepts no call chained on byteArrayOf; it
// matches a decode only by the text `Base64.decode(` or
// `Base64.getDecoder().decode(` plus a quote in the argument; and it needs the string argument to start with a quote
// directly followed by `.toByteArray(`, so a parenthesized concatenation
// (which K2 folds into one string) or a String transform before toByteArray is
// missed. FIR reads the resolved arguments instead.
package test

import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.IvParameterSpec
import kotlin.io.encoding.Base64
import java.util.Base64 as B64

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

    fun mimeDecoder() = <!StaticIv!>IvParameterSpec(java.util.Base64.getMimeDecoder().decode("AAAAAAAAAAAAAAAAAAAAAA=="))<!>

    // The decoded data is literal bytes, so the text holds no quote.
    fun decodedBytes() = <!StaticIv!>IvParameterSpec(java.util.Base64.getDecoder().decode(byteArrayOf(65, 65, 65, 65)))<!>

    fun parenthesizedConcatenation() = <!StaticIv!>IvParameterSpec(("01234567" + "89abcdef").toByteArray())<!>

    fun transformedText() = <!StaticIv!>IvParameterSpec(" 0123456789abcdef ".trim().toByteArray())<!>

    // An import alias hides the `Base64.getDecoder().decode(` text Go matches.
    fun aliasedDecoder() = <!StaticIv!>IvParameterSpec(B64.getDecoder().decode("AAAAAAAAAAAAAAAAAAAAAA=="))<!>

    fun heldDecoder(): IvParameterSpec {
        val decoder = java.util.Base64.getDecoder()
        return <!StaticIv!>IvParameterSpec(decoder.decode("AAAAAAAAAAAAAAAAAAAAAA=="))<!>
    }
}
