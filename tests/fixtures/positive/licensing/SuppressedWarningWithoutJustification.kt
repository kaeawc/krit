package licensing

@Suppress("UNCHECKED_CAST")
fun cast(map: Map<*, *>): Map<String, Int> {
    return map as Map<String, Int>
}
