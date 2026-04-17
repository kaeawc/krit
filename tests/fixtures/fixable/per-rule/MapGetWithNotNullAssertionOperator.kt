package potentialbugs

class MapGetWithNotNullAssertionOperator {
    fun getValue(map: Map<String, String>): String {
        return map["key"]!!
    }
}
