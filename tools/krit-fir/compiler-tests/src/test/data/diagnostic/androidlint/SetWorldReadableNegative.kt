// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives for SetWorldReadable: owner-only grants, revokes, unknown values, other methods and other types.
package test.setworldreadablenegative

import java.io.File

class Exporter(private val flag: Boolean) {
    var mutable = true

    fun ownerOnly(file: File) {
        file.setReadable(true, true)
    }

    fun revoke(file: File) {
        file.setReadable(false, false)
    }

    fun singleArgument(file: File) {
        file.setReadable(true)
    }

    fun unknown(file: File, permit: Boolean, ownerOnly: Boolean) {
        file.setReadable(permit, ownerOnly)
        file.setReadable(flag, false)
        file.setReadable(true, flag)
    }

    // A `var` is not a constant, as in Go.
    fun variable(file: File) {
        file.setReadable(mutable, false)
    }

    fun otherMethod(file: File) {
        file.setExecutable(true, false)
    }
}

// A project type with the same method name is not a java.io.File.
class Store {
    fun setReadable(permit: Boolean, ownerOnly: Boolean): Boolean = permit && ownerOnly
}

fun lookalike(store: Store) {
    store.setReadable(true, false)
}
