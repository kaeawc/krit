// Compiler-test source stubs; never packaged in the production artifact.
package android.database

import java.io.Closeable

interface Cursor : Closeable {
    // Java getCount()/isClosed(): read through Kotlin's synthetic properties.
    val count: Int

    val isClosed: Boolean

    fun moveToFirst(): Boolean

    fun moveToNext(): Boolean

    fun moveToPosition(position: Int): Boolean

    fun getColumnIndex(columnName: String): Int

    fun getColumnIndexOrThrow(columnName: String): Int

    fun getString(columnIndex: Int): String?

    fun getInt(columnIndex: Int): Int

    fun getLong(columnIndex: Int): Long

    fun getDouble(columnIndex: Int): Double

    fun getBlob(columnIndex: Int): ByteArray?

    fun isNull(columnIndex: Int): Boolean

    override fun close()
}

open class SQLException : RuntimeException {
    constructor()

    constructor(error: String?)
}
