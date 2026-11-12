// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 28, 33, 37, 39, 42
// Operand types the compiler knows and Go's source inference does not. Go judges the operand by its own
// inference and by name-based guards; the checker takes the type from the compiler.
package test.unsafecall.compiler

class Controller {
    fun start() {}
}

class Bundle {
    fun getString(key: String): String? = null
}

class Typed {
    private var controller: Controller? = null

    // A var property is not stable, so the assignment does not smart-cast it. Go reads the assignment as a guard.
    fun assignedProperty() {
        controller = Controller()
        controller<!UnsafeCallOnNullableType!>!!<!>.start()
    }

    // Go skips Bundle getters by name; the compiler types the result as nullable, so the assertion can fail.
    fun bundleGetter(bundle: Bundle): String = bundle.getString("key")<!UnsafeCallOnNullableType!>!!<!>

    // A platform type is not nullable, and Go cannot tell, so it reports.
    fun platformType(): String = System.getProperty("user.dir")!!

    // A copy made with an elvis is non-null.
    fun stableCopy(name: String?): Int {
        val copy = name ?: return 0
        return copy!!.length
    }

    // The declared type is non-null.
    fun notNull(name: String): Int = name!!.length

    fun <T : Any> bounded(value: T): T = value!!

    // A type parameter without a bound can be null.
    fun <T> unbounded(value: T): T = value<!UnsafeCallOnNullableType!>!!<!>
}
