package dev.krit.firsample.shared

import app.cash.sqldelight.db.SqlDriver
import dev.krit.firsample.shared.db.SampleDatabase

class NoteStore(driver: SqlDriver) {
    // SQLDelight generates SampleDatabase and its noteQueries from Note.sq, so
    // this file compiles only when the generated sources are on FIR's source path.
    private val queries = SampleDatabase(driver).noteQueries

    fun save(id: String, body: String, pinned: Boolean) {
        queries.insertNote(id, body, if (pinned) 1L else 0L)
    }

    fun pinned(): List<String> =
        queries.pinnedNotes().executeAsList().map { "${it.id}: ${it.body} (${platformName()})" }
}
