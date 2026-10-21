package fixtures.positive.kotlin24

class NamedContextLookup {
    // The map is a function parameter; the named context parameter is in scope
    // in the same function and does not hide the receiver's type.
    context(label: String)
    fun parameter(map: Map<String, String>): String {
        val call = map.get("key")!!
        val bracket = map["key"]!!
        return label + call + bracket
    }

    // The receiver is the context parameter itself.
    context(table: Map<String, String>)
    fun contextual(): String {
        val call = table.get("key")!!
        return call
    }
}
