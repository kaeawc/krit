package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOn

object db {
    fun query(): List<String> = listOf("row")
}

fun render(rows: List<String>) {}

fun collectRows() {
    flow {
        val rows = db.query()
        emit(rows)
    }.flowOn(Dispatchers.IO).collect { render(it) }
}
