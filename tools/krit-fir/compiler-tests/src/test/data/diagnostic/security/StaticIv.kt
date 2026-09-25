// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 15, 17, 19, 21, 23, 25, 27, 29, 31, 33, 35, 37, 39, 41, 43, 45, 47, 49, 51, 54, 62, 65, 67, 69, 72, 76, 80
// Positive: IvParameterSpec / GCMParameterSpec built from inline literal
// bytes. Go reports every call here on the line the call starts.
package test

import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.IvParameterSpec

const val IV_TEXT = "0123456789abcdef"

class Crypto {
    fun byteArray() = <!StaticIv!>IvParameterSpec(byteArrayOf(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0))<!>

    fun gcmString() = <!StaticIv!>GCMParameterSpec(128, "000000000000".toByteArray())<!>

    fun qualified() = <!StaticIv!>javax.crypto.spec.IvParameterSpec(kotlin.byteArrayOf(1, 2, 3))<!>

    fun signed() = <!StaticIv!>IvParameterSpec(byteArrayOf(-1, 0x10, +2, 1_0, 0X7f))<!>

    fun charset() = <!StaticIv!>IvParameterSpec("0123456789abcdef".toByteArray(Charsets.UTF_8))<!>

    fun encode() = <!StaticIv!>IvParameterSpec("0123456789abcdef".encodeToByteArray())<!>

    fun encodeRange() = <!StaticIv!>IvParameterSpec("0123456789abcdef!".encodeToByteArray(0, 16))<!>

    fun raw() = <!StaticIv!>IvParameterSpec("""0123456789abcdef""".toByteArray())<!>

    fun escaped() = <!StaticIv!>IvParameterSpec("0123456789abcd\n\t".toByteArray())<!>

    fun hex() = <!StaticIv!>IvParameterSpec("000102030405060708090a0b0c0d0e0f".hexToByteArray())<!>

    fun constTemplate() = <!StaticIv!>IvParameterSpec("$IV_TEXT".toByteArray())<!>

    fun javaDecoder() = <!StaticIv!>IvParameterSpec(java.util.Base64.getDecoder().decode("AAAAAAAAAAAAAAAAAAAAAA=="))<!>

    fun javaDecoderBytes() = <!StaticIv!>IvParameterSpec(java.util.Base64.getDecoder().decode("AAAAAAAAAAAAAAAAAAAAAA==".toByteArray()))<!>

    fun kotlinBase64() = <!StaticIv!>IvParameterSpec(kotlin.io.encoding.Base64.decode("AAAAAAAAAAAAAAAAAAAAAA=="))<!>

    fun ivRange() = <!StaticIv!>IvParameterSpec(byteArrayOf(1, 2, 3), 0, 2)<!>

    fun gcmRange() = <!StaticIv!>GCMParameterSpec(128, byteArrayOf(1, 2, 3), 0, 3)<!>

    fun parenthesized() = <!StaticIv!>IvParameterSpec(("0123456789abcdef".toByteArray()))<!>

    fun copied() = <!StaticIv!>IvParameterSpec("0123456789".toByteArray().copyOf(16))<!>

    fun concatenated() = <!StaticIv!>IvParameterSpec("01234567".toByteArray() + "89abcdef".toByteArray())<!>

    fun decodedCopy() = <!StaticIv!>IvParameterSpec(java.util.Base64.getDecoder().decode("AAAA").copyOf(16))<!>

    fun multiline(): IvParameterSpec =
        <!StaticIv!>IvParameterSpec<!>(
            byteArrayOf(
                1,
                2
            ),
        )

    fun qualifiedMultiline(): IvParameterSpec =
        <!StaticIv!>javax<!>.crypto.spec
            .IvParameterSpec(byteArrayOf(1))

    fun nested(): ByteArray = <!StaticIv!>IvParameterSpec(byteArrayOf(1))<!>.iv

    val property = <!StaticIv!>IvParameterSpec(byteArrayOf(1, 2))<!>

    fun inLambda() = listOf(1).map { <!StaticIv!>IvParameterSpec(byteArrayOf(1))<!> }

    fun inObject() = object {
        fun spec() = <!StaticIv!>IvParameterSpec(byteArrayOf(1))<!>
    }

    companion object {
        val SPEC = <!StaticIv!>GCMParameterSpec(128, "000000000000".toByteArray())<!>
    }
}

fun topLevel() = <!StaticIv!>IvParameterSpec(byteArrayOf(1))<!>
