package test

annotation class Database(
    val entities: Array<Any> = [],
    val version: Int = 1,
    val exportSchema: Boolean = true,
)

class RoomDatabase

class User

@Database(entities = [User::class], version = 3, exportSchema = false)
abstract class AppDb : RoomDatabase()
