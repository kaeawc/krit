package style

class Outer {
    val x = 1
    inner class Inner {
        fun foo() = this@Outer.x
    }
}
