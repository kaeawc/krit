package potentialbugs

class Holder(val maps: Maps)
class Maps(val current: Map<String, String>)

class MapGetWithNotNullAssertionOperator {
    fun getValue(map: Map<String, String>, holder: Holder): String {
        val bracket = map["key"]!!
        val call = map.get("key")!!
        val nested = holder.maps.current["key"]!!
        return bracket + call + nested
    }
}
