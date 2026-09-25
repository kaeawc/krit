// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: an IV template over a property initialized with a literal
// string. A var or an open val counts: the literal it is initialized with is
// written in the source, even if the value can later be reassigned or
// overridden. Go reports each of these because the argument starts with a
// string literal followed by `.toByteArray(`.
package test

import javax.crypto.spec.IvParameterSpec

var MUTABLE_PREFIX = "0123456789"

open class Config {
    open val prefix = "0123456789"
}

class Crypto(private val config: Config) {
    fun varTemplate() = <!StaticIv!>IvParameterSpec("$MUTABLE_PREFIX-abcde".toByteArray())<!>

    fun openValTemplate() = <!StaticIv!>IvParameterSpec("${config.prefix}-abcde".toByteArray())<!>

    fun localVarTemplate(): IvParameterSpec {
        var local = "0123456789"
        local += "ab"
        return <!StaticIv!>IvParameterSpec("$local-abcd".toByteArray())<!>
    }
}
