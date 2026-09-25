// RENDER_DIAGNOSTICS_FULL_TEXT
// Deliberate improvements: an import alias or a typealias still constructs
// javax.crypto.spec.IvParameterSpec / GCMParameterSpec, so these are real
// findings. Go misses them because it requires the call's simple name to be
// IvParameterSpec or GCMParameterSpec; FIR reads the constructed class. A
// class nested elsewhere in the file that shares the simple name does not
// shadow the import, but Go skips every bare call once the file declares one.
package test

import javax.crypto.spec.GCMParameterSpec as Gcm
import javax.crypto.spec.IvParameterSpec

typealias Iv = IvParameterSpec

class Holder {
    class IvParameterSpec(val bytes: ByteArray)

    fun nested() = IvParameterSpec(byteArrayOf(1, 2))
}

class Crypto {
    fun importAlias() = <!StaticIv!>Gcm(128, "000000000000".toByteArray())<!>

    fun typeAlias() = <!StaticIv!>Iv(byteArrayOf(1, 2, 3))<!>

    fun nestedLookalikeElsewhere() = <!StaticIv!>IvParameterSpec(byteArrayOf(1, 2, 3))<!>
}
