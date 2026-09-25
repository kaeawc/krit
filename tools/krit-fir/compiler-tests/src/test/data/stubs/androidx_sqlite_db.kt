// Compiler-test source stubs; never packaged in the production artifact.
package androidx.sqlite.db

import android.content.ContentValues
import android.database.Cursor
import java.io.Closeable

interface SupportSQLiteDatabase : Closeable {
    val isOpen: Boolean

    var version: Int

    fun execSQL(sql: String)

    fun execSQL(sql: String, bindArgs: Array<out Any?>)

    fun query(query: String): Cursor

    fun query(query: String, bindArgs: Array<out Any?>): Cursor

    fun insert(table: String, conflictAlgorithm: Int, values: ContentValues): Long

    fun update(
        table: String,
        conflictAlgorithm: Int,
        values: ContentValues,
        whereClause: String?,
        whereArgs: Array<out Any?>?,
    ): Int

    fun delete(table: String, whereClause: String?, whereArgs: Array<out Any?>?): Int

    fun compileStatement(sql: String): SupportSQLiteStatement

    fun beginTransaction()

    fun setTransactionSuccessful()

    fun endTransaction()

    fun inTransaction(): Boolean
}

interface SupportSQLiteStatement : Closeable {
    fun bindNull(index: Int)

    fun bindLong(index: Int, value: Long)

    fun bindString(index: Int, value: String)

    fun execute()

    fun executeInsert(): Long

    fun executeUpdateDelete(): Int
}

inline fun <T> SupportSQLiteDatabase.transaction(
    exclusive: Boolean = true,
    body: SupportSQLiteDatabase.() -> T,
): T = TODO()
