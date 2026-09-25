// Smoke: a Migration subclass with its version range.
package stubs

import androidx.room.migration.Migration
import androidx.sqlite.db.SupportSQLiteDatabase

class Migration2To3 : Migration(2, 3) {
    override fun migrate(db: SupportSQLiteDatabase) {
        db.execSQL("CREATE TABLE IF NOT EXISTS audit (id INTEGER PRIMARY KEY)")
        <!PrintlnInProduction!>println<!>("$startVersion -> $endVersion")
    }
}
