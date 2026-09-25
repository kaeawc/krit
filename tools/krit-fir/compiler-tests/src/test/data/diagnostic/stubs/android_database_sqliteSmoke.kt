// Smoke: SQLiteOpenHelper subclass plus raw SQLiteDatabase transactions.
package stubs

import android.content.ContentValues
import android.content.Context
import android.database.sqlite.SQLiteDatabase
import android.database.sqlite.SQLiteOpenHelper

class SmokeHelper(context: Context) : SQLiteOpenHelper(context, "smoke.db", null, 1) {
    override fun onCreate(db: SQLiteDatabase) {
        db.execSQL("CREATE TABLE t (id INTEGER PRIMARY KEY)")
    }

    override fun onUpgrade(db: SQLiteDatabase, oldVersion: Int, newVersion: Int) {
        db.execSQL("DROP TABLE t")
        onCreate(db)
    }
}

fun insertRow(helper: SmokeHelper) {
    val db: SQLiteDatabase = helper.writableDatabase
    db.beginTransaction()
    try {
        db.insert("t", null, ContentValues().apply { put("id", 1) })
        db.update("t", ContentValues(), "id = ?", arrayOf("1"))
        db.delete("t", "id = ?", arrayOf("2"))
        db.rawQuery("SELECT * FROM t WHERE id = ?", arrayOf("1")).use { it.moveToFirst() }
        db.query("t", null, null, null, null, null, null).close()
        db.execSQL("DELETE FROM t WHERE id = ?", arrayOf<Any?>(3))
        db.setTransactionSuccessful()
    } finally {
        db.endTransaction()
    }
    if (db.isOpen) helper.close()
}
