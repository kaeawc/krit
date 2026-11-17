// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 35
// Divergences from Go, which types the receiver from declarations it can read.
package test.setworldreadabledivergence

import java.io.File as JavaFile

typealias Path = java.io.File

class Source {
    val target: java.io.File get() = error("stub")
    fun make(): java.io.File = error("stub")
}

// Go resolves an import alias through the file's imports (it reports this); it misses a type alias of File.
fun importAlias(file: JavaFile) {
    <!SetWorldReadable!>file.setReadable(true, false)<!>
}

fun typeAlias(file: Path) {
    <!SetWorldReadable!>file.setReadable(true, false)<!>
}

// Go cannot type a property of a project class or a call result.
fun fromProperty(source: Source) {
    <!SetWorldReadable!>source.target.setReadable(true, false)<!>
}

fun fromCall(source: Source) {
    <!SetWorldReadable!>source.make().setReadable(true, false)<!>
}

// Go reads through `!!` (it reports this) but cannot type a smart-cast receiver.
fun notNull(file: java.io.File?) {
    <!SetWorldReadable!>file!!.setReadable(true, false)<!>
}

fun smartCast(file: Any) {
    if (file is java.io.File) {
        <!SetWorldReadable!>file.setReadable(true, false)<!>
    }
}

// Android Lint reports a call on a File subclass that overrides setReadable; Go does not type the subclass.
class LoggingFile(path: String) : java.io.File(path) {
    override fun setReadable(readable: Boolean, ownerOnly: Boolean): Boolean = super.setReadable(readable, ownerOnly)
}

fun subclassOverride(file: LoggingFile) {
    <!SetWorldReadable!>file.setReadable(true, false)<!>
}

// Android Lint folds `&&` and `||` of constants; Go does not.
fun logicOperators(file: java.io.File) {
    <!SetWorldReadable!>file.setReadable(true && !false, false || false)<!>
}
