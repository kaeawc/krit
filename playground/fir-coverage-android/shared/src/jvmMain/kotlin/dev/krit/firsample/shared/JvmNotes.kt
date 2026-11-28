package dev.krit.firsample.shared

import app.cash.sqldelight.driver.jdbc.sqlite.JdbcSqliteDriver
import com.squareup.moshi.JsonClass
import com.squareup.moshi.Moshi
import dev.krit.firsample.shared.db.SampleDatabase

// Moshi's KSP processor (kspJvm) generates NoteJsonJsonAdapter for this class.
@JsonClass(generateAdapter = true)
data class NoteJson(val id: String, val body: String, val pinned: Boolean = false)

object JvmNotes {
    // Refers to the KSP-generated adapter directly, so this file compiles
    // only when the jvm target's generated sources are on FIR's source path.
    private val adapter = NoteJsonJsonAdapter(Moshi.Builder().build())

    fun inMemoryStore(): NoteStore {
        val driver = JdbcSqliteDriver(JdbcSqliteDriver.IN_MEMORY)
        SampleDatabase.Schema.create(driver)
        return NoteStore(driver)
    }

    fun import(store: NoteStore, json: String) {
        val note = adapter.fromJson(json) ?: return
        store.save(note.id, note.body, note.pinned)
    }
}
