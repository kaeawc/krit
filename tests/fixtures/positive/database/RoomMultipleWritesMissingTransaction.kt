package test

annotation class Dao
annotation class Insert
annotation class Update
annotation class Delete
annotation class Transaction

data class User(val id: Int)
data class Prefs(val id: Int)

@Dao
interface UserDao {
    @Insert
    fun insertUser(user: User)

    @Insert
    fun insertPrefs(prefs: Prefs)

    @Suppress("DaoWithoutAnnotations")
    fun save(user: User, prefs: Prefs) {
        insertUser(user)
        insertPrefs(prefs)
    }
}
