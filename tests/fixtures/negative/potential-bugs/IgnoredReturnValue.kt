package fixtures.negative.potentialbugs

class IgnoredReturnValue {
    fun process(list: List<Int>): List<Int> {
        val result = list.map { it * 2 }
        return result
    }

    fun tryExpressionUsed(list: List<String>): Set<String> {
        return try {
            list.toSet()
        } catch (e: Exception) {
            emptySet()
        }
    }
}
