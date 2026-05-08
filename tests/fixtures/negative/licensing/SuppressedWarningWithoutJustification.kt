package licensing

/** Safe: the map is produced by a factory that always returns String to Int. */
@Suppress("UNCHECKED_CAST")
fun cast(map: Map<*, *>): Map<String, Int> {
    return map as Map<String, Int>
}
