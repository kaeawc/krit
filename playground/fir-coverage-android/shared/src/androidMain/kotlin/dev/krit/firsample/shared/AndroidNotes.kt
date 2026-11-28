package dev.krit.firsample.shared

import android.content.Context
import app.cash.sqldelight.driver.android.AndroidSqliteDriver
import com.squareup.moshi.JsonClass
import com.squareup.moshi.Moshi
import dev.krit.firsample.shared.db.SampleDatabase

// Moshi's KSP processor (kspAndroid) generates CachedNoteJsonAdapter for this class.
@JsonClass(generateAdapter = true)
data class CachedNote(val id: String, val body: String)

class AndroidNotes(context: Context) {
    // Refers to the KSP-generated adapter directly, so this file compiles
    // only when the Android target's generated sources are on FIR's source path.
    private val adapter = CachedNoteJsonAdapter(Moshi.Builder().build())
    val store = NoteStore(AndroidSqliteDriver(SampleDatabase.Schema, context, "notes.db"))

    fun restore(json: String) {
        val note = adapter.fromJson(json) ?: return
        store.save(note.id, note.body, pinned = true)
    }
}
