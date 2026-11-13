// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 14, 20, 26, 32, 38, 43, 50, 58, 61, 67
// Positives: a local or private var that nothing reassigns.
package test.varcouldbeval

class Box(var x: Int)

fun local() {
    <!VarCouldBeVal!>var<!> x = 1
    println(x)
}

fun typed() {
    <!VarCouldBeVal!>var<!> name: String = "a"
    println(name)
}

fun inLambda() {
    run {
        <!VarCouldBeVal!>var<!> inner = 2
        println(inner)
    }
}

class Holder {
    <!VarCouldBeVal!>private<!> var unused = 42
    fun read() = unused

    private val peer = Box(1)

    fun writesOtherReceiver() {
        <!VarCouldBeVal!>var<!> x = 0
        peer.x = 42
        println(x)
    }
}

<!VarCouldBeVal!>private<!> var topLevel = "t"

fun readTop() = topLevel

object Config {
    <!VarCouldBeVal!>private<!> var enabled = false

    fun read() = Config.enabled
}

class WithCompanion {
    companion object {
        <!VarCouldBeVal!>private<!> var shared = 0
    }

    fun read() = shared
}

class Annotated {
    <!VarCouldBeVal!>@Deprecated("x")<!>
    private var tagged = 1

    /** Doc comment before the declaration. */
    <!VarCouldBeVal!>private<!> var documented = 2

    fun read() = tagged + documented
}

class Lateinit {
    <!VarCouldBeVal!>private<!> lateinit var neverSet: String

    fun read() = neverSet
}
