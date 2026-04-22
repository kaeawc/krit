package potentialbugs

class MapGetWithNotNullAssertionOperator {
    fun getValue(map: Map<String, String>): String {
        val bracket = map["key"]!!
        val call = map.get("other")!!
        return bracket + call
    }
}
