package test

annotation class Entity(val tableName: String = "")
annotation class PrimaryKey

abstract class Migration(val from: Int, val to: Int) {
    abstract fun migrate(db: SupportSQLiteDatabase)
}

interface SupportSQLiteDatabase {
    fun execSQL(sql: String)
}

@Entity(tableName = "users")
data class User(
    @PrimaryKey val id: Long,
    val name: String,
    val avatarUrl: String,
)

object Migration1to2 : Migration(1, 2) {
    override fun migrate(db: SupportSQLiteDatabase) {
        db.execSQL("ALTER TABLE users ADD COLUMN id INTEGER")
        db.execSQL("ALTER TABLE users ADD COLUMN name TEXT")
        db.execSQL("ALTER TABLE users ADD COLUMN avatarUrl TEXT")
    }
}
