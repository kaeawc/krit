// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: the constructor resolves to javax.crypto.spec.SecretKeySpec under
// spellings the Go rule does not recognize.
package test

import javax.crypto.spec.SecretKeySpec
import javax.crypto.spec.SecretKeySpec as KeySpec

typealias AesKeySpec = SecretKeySpec

// An unrelated nested class that shares the simple name.
class Registry {
    class SecretKeySpec(val name: String)
}

class Crypto {
    // Go misses these because the call is not spelled `SecretKeySpec`; FIR is
    // correct because each resolves to javax.crypto.spec.SecretKeySpec.
    fun aliased() {
        <!HardcodedSecretKey!>KeySpec<!>(byteArrayOf(1, 2, 3, 4), "AES")
        <!HardcodedSecretKey!>AesKeySpec<!>("p@ssw0rd12345678".toByteArray(), "AES")
    }

    // Go misses this because the file declares a class named SecretKeySpec
    // (Registry.SecretKeySpec), which disables its import check for the whole
    // file; FIR is correct because the bare call resolves to the imported class.
    fun importedDespiteNestedName() {
        <!HardcodedSecretKey!>SecretKeySpec<!>(byteArrayOf(1, 2, 3, 4), "AES")
        Registry.SecretKeySpec("not a key")
    }
}
