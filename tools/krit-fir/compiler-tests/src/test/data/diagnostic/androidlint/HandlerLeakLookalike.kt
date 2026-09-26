// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Lookalike Handler classes that are not android.os.Handler.
package test

open class Handler {
    open class Base
}

open class MyHandler

class Screen {
    inner class SameName : Handler()

    inner class Nested : Handler.Base()

    inner class Local : MyHandler()

    val anonymous = object : Handler() {}
}
