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

    fun use() = publicProp + withSetter
}
