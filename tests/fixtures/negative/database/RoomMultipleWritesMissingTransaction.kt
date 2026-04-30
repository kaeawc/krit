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

    @Update
    fun updateUser(user: User)

    @Transaction
    fun save(user: User, prefs: Prefs) {
        insertUser(user)
        insertPrefs(prefs)
    }

    fun singleWrite(user: User) {
        insertUser(user)
    }

    fun nonDaoCalls() {
        println("hello")
        println("world")
    }
}

class NotADao {
    fun save(user: User, prefs: Prefs) {
        insertSomething(user)
        insertOther(prefs)
    }

    fun insertSomething(user: User) {}
    fun insertOther(prefs: Prefs) {}
}
