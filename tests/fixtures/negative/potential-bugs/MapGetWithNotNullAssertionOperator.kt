package potentialbugs

class Box {
    operator fun get(key: String): String? = null
}

class MapGetWithNotNullAssertionOperator {
    fun getValue(map: Map<String, String>, box: Box): String {
        if (map.containsKey("key")) {
            val guarded = map["key"]!!
        }
        val nonMapIndex = box["key"]!!
        return map.getValue("key") + nonMapIndex
    }
}
