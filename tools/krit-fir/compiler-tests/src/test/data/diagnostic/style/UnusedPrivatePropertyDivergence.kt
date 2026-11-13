// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Shapes where Go and the checker disagree.
package test.unusedprivatepropertydivergence

class First {
    // Go keeps this quiet because the name `shared` appears in `Second` below; they are different properties.
    private val <!UnusedPrivateProperty!>shared<!> = 1
}

class Second {
    private val shared = 2
    fun read() = shared
}

class ParameterNamed {
    // Go keeps this quiet because a parameter of `use` has the same name.
    private val <!UnusedPrivateProperty!>count<!> = 1
    fun use(count: Int) = count + 1
}

class LocalNamed {
    // Go keeps this quiet because a local of the same name is read.
    private val <!UnusedPrivateProperty!>total<!> = 1
    fun use(): Int {
        val total = 2
        return total
    }
}

class CallNamed {
    // Go keeps this quiet because a call to a method of the same name appears.
    private val <!UnusedPrivateProperty!>size<!> = 1
    fun use(list: List<Int>) = list.size
}

class ConstructorShadow(plain: Int) {
    // Go keeps both quiet: each name appears twice. Only the property is unused.
    private val <!UnusedPrivateProperty!>plain<!> = plain
}

class ThroughExtension {
    // Go keeps this quiet because `name` appears as a member access on another receiver.
    private val <!UnusedPrivateProperty!>name<!> = "n"
    fun use(other: Named) = other.name
}

class Named(val name: String)

// Go keeps this parameter quiet because the name `name` appears elsewhere in the file as a member access.
class PlainNamedLikeMember(<!UnusedPrivateProperty!>name<!>: String)
