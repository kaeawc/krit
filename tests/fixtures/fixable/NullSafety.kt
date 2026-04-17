package com.example.nullsafety

class NullSafety {

    fun nullChecks(x: String?, y: Int?) {
        check(x != null)
        require(y != null)
    }

    fun equalsNull(obj: Any?) {
        val isNull = obj.equals(null)
    }

    fun nullOrEmpty(x: String?) {
        if (x == null || x.isEmpty()) {
            println("missing")
        }
    }

    fun orEmptyFallbacks(list: List<String>?, str: String?, set: Set<Int>?, map: Map<String, Int>?) {
        val safeList = list ?: emptyList()
        val safeSet = set ?: emptySet()
        val safeMap = map ?: emptyMap()
    }
}
