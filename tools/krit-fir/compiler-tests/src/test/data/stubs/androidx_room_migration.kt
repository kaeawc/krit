// Compiler-test source stubs; never packaged in the production artifact.
package androidx.room.migration

import androidx.sqlite.db.SupportSQLiteDatabase

abstract class Migration(
    @JvmField val startVersion: Int,
    @JvmField val endVersion: Int,
) {
    abstract fun migrate(db: SupportSQLiteDatabase)
}

interface AutoMigrationSpec {
    fun onPostMigrate(db: SupportSQLiteDatabase) {}
}
