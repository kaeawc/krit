package fixtures.negative.style

fun compute() {
    var x = 1
    x = 2
    println(x)

    var y = 0
    y += 10

    var z = 5
    z++

    var w = 3
    --w

    // Delegate: var controls mutability via delegate
    var delegated by lazy { 1 }

    println(y + z + w + delegated)
}

class Container {
    // Non-private class property: may be reassigned externally
    var publicProp = 1

    // Override: changing to val would break the contract
    // (commented out since we lack the interface here)
    // override var overriddenProp = 1

    // Private with custom setter
    private var withSetter: Int = 0
        set(value) { field = value }

    private var listener: (() -> Unit)? = null

    fun setListener(value: (() -> Unit)?) {
        this.listener = value
    }

    private var builderFlag = false

    fun enable(): Container {
        this.builderFlag = true
        return this
    }

    fun use() = publicProp + withSetter
}

// lateinit var cannot be val — must never be flagged.
object LateinitHolder {
    private lateinit var provider: String

    fun init(value: String) {
        provider = value
    }

    fun read() = provider
}

// Member reassigned through a qualified write `EnclosingType.member = ...`
// from a sibling nested scope outside the member's immediate class_body.
class ProgressService {
    companion object {
        private var title: String = ""
    }

    class Controller {
        fun update(newTitle: String) {
            ProgressService.title = newTitle
        }
    }
}

// Object member reassigned through its own object name.
object Deps {
    private var configured = false

    fun configure() {
        Deps.configured = true
    }

    fun read() = configured
}
