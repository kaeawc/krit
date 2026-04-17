package test

import kotlinx.coroutines.flow.flow

object db {
    fun query(): List<String> = listOf("row")
}

fun render(rows: List<String>) {}

fun collectRows() {
    flow {
        val rows = db.query()
        emit(rows)
    }.collect { render(it) }
}
