// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: IVs that are not inline literal bytes, other spec classes, and
// local lookalikes. Neither Go nor FIR reports anything here.
package test

import java.security.SecureRandom
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.IvParameterSpec

const val IV_BYTE: Byte = 1
const val IV_TEXT = "0123456789abcdef"
val IV_BYTES = byteArrayOf(1, 2, 3)

object Local {
    class IvParameterSpec(val bytes: ByteArray)
}

class Crypto(private val field: ByteArray) {
    fun params(param: ByteArray, text: String) {
        val random = ByteArray(16)
        SecureRandom().nextBytes(random)
        IvParameterSpec(param)
        IvParameterSpec(field)
        IvParameterSpec(random)
        IvParameterSpec(ByteArray(16))
        IvParameterSpec(IV_BYTES)
        IvParameterSpec(text.toByteArray())
        IvParameterSpec(IV_TEXT.toByteArray())
        IvParameterSpec(byteArrayOf())
        IvParameterSpec(byteArrayOf(*random))
        IvParameterSpec(byteArrayOf(IV_BYTE, 2))
        IvParameterSpec(byteArrayOf((1 + 2).toByte()))
        IvParameterSpec(byteArrayOf(0xFF.toByte()))
        IvParameterSpec(random.copyOf(16))
        IvParameterSpec(java.util.Base64.getDecoder().decode(text))
        IvParameterSpec(java.util.Base64.getDecoder().decode(IV_TEXT))
        GCMParameterSpec(128, random)
        GCMParameterSpec(byteArrayOf(1, 2).size * 64, random)
        Local.IvParameterSpec(byteArrayOf(0, 0, 0, 0))
    }
}
