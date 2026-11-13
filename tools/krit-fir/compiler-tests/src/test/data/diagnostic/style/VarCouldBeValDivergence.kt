// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 34, 41
// Shapes where Go and the checker disagree.
package test.varcouldbevaldivergence

class Box(var x: Int)

// Go reads the loop's `total += v` as a write to the property of the same name. The local is a different
// symbol, so the property is never written and the checker reports it.
class Shadowed {
    <!VarCouldBeVal!>private<!> var total = 0

    fun sum(values: List<Int>): Int {
        var total = 0
        for (v in values) total += v
        return total
    }

    fun read() = total
}

// Go counts a write to `x` through any receiver, so a write to another class's `x` hides the property.
class Sibling {
    <!VarCouldBeVal!>private<!> var x = 1

    fun read(box: Box): Int {
        box.x = 5
        return x
    }
}

// Go reports a property whose only use is a bound callable reference, which can write it.
class Referenced {
    private var slot = 0

    fun reference() = this::slot
}

// Go does not see `field++` as a write to the property, so it reports it.
class GetterField {
    private var cached = 0
        get() {
            field++
            return field
        }

    fun read() = cached
}
