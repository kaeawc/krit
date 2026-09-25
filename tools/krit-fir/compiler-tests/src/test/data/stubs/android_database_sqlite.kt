// Compiler-test source stubs; never packaged in the production artifact.
package android.database.sqlite

import android.content.ContentValues
import android.content.Context
import android.database.Cursor
import java.io.Closeable

abstract class SQLiteClosable : Closeable {
    override fun close() {
        TODO()
    }
}

// The framework SQLiteDatabase does NOT implement AndroidX
// SupportSQLiteDatabase; they are unrelated types.
class SQLiteDatabase private constructor() : SQLiteClosable() {
    val isOpen: Boolean
        get() = TODO()

    val version: Int
        get() = TODO()

    fun execSQL(sql: String) {
        TODO()
    }

    fun execSQL(sql: String, bindArgs: Array<Any?>) {
        TODO()
    }

    fun rawQuery(sql: String, selectionArgs: Array<String>?): Cursor = TODO()

    fun query(
        table: String,
        columns: Array<String>?,
        selection: String?,
        selectionArgs: Array<String>?,
        groupBy: String?,
        having: String?,
        orderBy: String?,
    ): Cursor = TODO()

    fun insert(table: String, nullColumnHack: String?, values: ContentValues?): Long = TODO()

    fun update(table: String, values: ContentValues?, whereClause: String?, whereArgs: Array<String>?): Int = TODO()

    fun delete(table: String, whereClause: String?, whereArgs: Array<String>?): Int = TODO()

    fun beginTransaction() {
        TODO()
    }

    fun setTransactionSuccessful() {
        TODO()
    }

    fun endTransaction() {
        TODO()
    }

    fun inTransaction(): Boolean = TODO()

    fun interface CursorFactory {
        fun newCursor(db: SQLiteDatabase, masterQuery: Any?, editTable: String?, query: Any?): Cursor
    }

    companion object {
        const val CONFLICT_REPLACE: Int = 5

        fun openOrCreateDatabase(path: String, factory: CursorFactory?): SQLiteDatabase = TODO()
    }
}

abstract class SQLiteOpenHelper(
    context: Context?,
    name: String?,
    factory: SQLiteDatabase.CursorFactory?,
    version: Int,
) : AutoCloseable {
    open val writableDatabase: SQLiteDatabase
        get() = TODO()

    open val readableDatabase: SQLiteDatabase
        get() = TODO()

    abstract fun onCreate(db: SQLiteDatabase)

    abstract fun onUpgrade(db: SQLiteDatabase, oldVersion: Int, newVersion: Int)

    open fun onOpen(db: SQLiteDatabase) {
        TODO()
    }

    override fun close() {
        TODO()
    }
}
