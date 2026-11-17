// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives for SetWorldWritable: owner-only grants, revokes, unknown values, other methods and other types.
package test.setworldwritablenegative

import java.io.File

class Exporter(private val flag: Boolean) {
    var mutable = true

    fun ownerOnly(file: File) {
        file.setWritable(true, true)
    }

    fun revoke(file: File) {
        file.setWritable(false, false)
    }

    fun singleArgument(file: File) {
        file.setWritable(true)
    }

    fun unknown(file: File, permit: Boolean, ownerOnly: Boolean) {
        file.setWritable(permit, ownerOnly)
        file.setWritable(flag, false)
        file.setWritable(true, flag)
    }

    // A `var` is not a constant, as in Go.
    fun variable(file: File) {
        file.setWritable(mutable, false)
    }

    fun otherMethod(file: File) {
        file.setExecutable(true, false)
    }
}

// A project type with the same method name is not a java.io.File.
class Store {
    fun setWritable(permit: Boolean, ownerOnly: Boolean): Boolean = permit && ownerOnly
}

fun lookalike(store: Store) {
    store.setWritable(true, false)
}
