package fixtures.negative.kotlin24

class Box {
    operator fun get(key: String): String? = null
}

class NamedContextLookup {
    // A context parameter that is not a map: its `get` is not Map.get.
    context(box: Box)
    fun contextual(): String {
        return box["key"]!!
    }

    // A local shadows the context parameter name with a non-map value.
    context(table: Map<String, String>)
    fun shadowed(): String {
        val table = Box()
        return table["key"]!!
    }

    // Guarded by containsKey.
    context(table: Map<String, String>)
    fun guarded(): String {
        if (table.containsKey("key")) {
            return table.getValue("key")
        }
        return ""
    }
}
