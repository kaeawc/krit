// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 16, 18, 20, 22, 32, 36
// Go findings FIR drops: none of these properties has a mutable collection
// type, so Go's message ("Variable with mutable collection type ...") is false
// of the code. Go falls back to the initializer's call name when the declared
// type is not a mutable collection name, so it reports any `var` initialized
// by a call named like a mutable collection factory, whatever the declared
// type and whatever the call resolves to.
package test

class Divergence {
    // Go reports these: the declared type is read-only; the initializer's
    // mutable collection is only reachable through a cast.
    var readOnly: List<String> = mutableListOf()

    var readOnlyCollection: Collection<String> = ArrayList()

    var readOnlyMap: Map<String, Int> = hashMapOf()

    var nullableIterable: Iterable<String>? = linkedSetOf()

    var anything: Any = mutableSetOf<String>()
}

class Builder {
    fun mutableListOf(): Int = 0

    class HashMap(val capacity: Int)

    // Go reports this: the call resolves to the member above, which returns
    // Int, not a collection.
    var count = mutableListOf()

    // Go reports this: the call builds the nested Builder.HashMap, which is not
    // a collection.
    var made = HashMap(16)
}
