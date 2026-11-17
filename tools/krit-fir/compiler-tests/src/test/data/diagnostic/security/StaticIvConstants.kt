// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Deliberate improvements: IVs whose bytes are fixed in the source but that Go
// does not match. Go needs the argument text to start with `byteArrayOf(`, a
// quote, or a Base64 decode with a quote in its argument, so it misses a
// zero-filled `ByteArray(size)` and toByteArray or decode called on a String
// constant or literal-initialized property itself. FIR already reports a
// template over such a property (`"$IV_TEXT".toByteArray()`); the bare
// property holds the same bytes.
package test

import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.IvParameterSpec

const val IV_TEXT = "0123456789abcdef"
private val IV_PROPERTY = "fedcba9876543210"
private const val IV_BASE64 = "AAAAAAAAAAAAAAAAAAAAAA=="

class Crypto(private val blockSize: Int) {
    fun zeros() = <!StaticIv!>IvParameterSpec(ByteArray(16))<!>

    fun zerosOfSize() = <!StaticIv!>IvParameterSpec(ByteArray(blockSize))<!>

    fun gcmZeros() = <!StaticIv!>GCMParameterSpec(128, ByteArray(12))<!>

    fun zerosCopy() = <!StaticIv!>IvParameterSpec(ByteArray(16).copyOf(12))<!>

    fun constant() = <!StaticIv!>IvParameterSpec(IV_TEXT.toByteArray())<!>

    fun constantCharset() = <!StaticIv!>IvParameterSpec(IV_TEXT.toByteArray(Charsets.UTF_8))<!>

    fun property() = <!StaticIv!>IvParameterSpec(IV_PROPERTY.encodeToByteArray())<!>

    fun transformed() = <!StaticIv!>IvParameterSpec(IV_TEXT.trimEnd().toByteArray())<!>

    fun javaDecodeConstant() = <!StaticIv!>IvParameterSpec(java.util.Base64.getDecoder().decode(IV_BASE64))<!>
}
