package performance

import kotlinx.coroutines.flow.Flow

class Query<T> {
    fun filter(block: (T) -> Boolean): Query<T> = this
    fun map(block: (T) -> Int): Query<T> = this
    fun take(count: Int): Query<T> = this
}

fun shortChain(list: List<Int>): List<Int> {
    return list.map { it * 2 }
}

fun existingSequence(items: List<Int>): List<Int> {
    return items
        .asSequence()
        .filter { it > 1 }
        .map { it * 2 }
        .toList()
}

fun flowChain(flow: Flow<Int>): Flow<Int> {
    return flow
        .filter { it > 1 }
        .map { it * 2 }
        .take(10)
}

fun customFluent(query: Query<Int>): Query<Int> {
    return query
        .filter { true }
        .map { it }
        .take(10)
}

fun mapSource(): List<String> {
    return mapOf("a" to 1, "b" to 2)
        .filter { it.value > 1 }
        .map { it.key }
        .sorted()
}
