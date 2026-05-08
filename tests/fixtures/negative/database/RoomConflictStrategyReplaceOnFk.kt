package test

annotation class Entity(val tableName: String = "", val foreignKeys: Array<ForeignKey> = [])
annotation class ForeignKey(val parent: kotlin.reflect.KClass<*>)
annotation class Insert(val onConflict: Int = 1)
annotation class Dao

object OnConflictStrategy {
    const val REPLACE = 1
    const val IGNORE = 2
}

@Entity
class Team(val id: Long)

@Entity(foreignKeys = [ForeignKey(parent = Team::class)])
class User(val id: Long, val teamId: Long)

@Entity
class Standalone(val id: Long)

@Dao
interface UserDao {
    @Insert(onConflict = OnConflictStrategy.IGNORE)
    suspend fun insertUser(user: User)

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertStandalone(item: Standalone)
}
