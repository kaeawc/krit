package test

abstract class Migration(val startVersion: Int, val endVersion: Int) {
    abstract fun migrate(db: SupportSQLiteDatabase)
}

interface SupportSQLiteDatabase {
    fun execSQL(sql: String)
}

object Migration1to2 : Migration(1, 2) {
    override fun migrate(db: SupportSQLiteDatabase) {
        val tableName = "users"
        db.execSQL("ALTER TABLE ${tableName} ADD COLUMN foo TEXT")
    }
}
