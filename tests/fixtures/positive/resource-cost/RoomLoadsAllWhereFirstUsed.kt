package fixtures.positive.resourcecost

annotation class Dao
annotation class Query(val value: String)

class User

@Dao
interface UserDao {
    @Query("SELECT * FROM users")
    fun getAll(): List<User>
}

class RoomLoadsAllWhereFirstUsed(private val dao: UserDao) {
    fun getFirstUser(dao: UserDao): Any {
        return dao.getAll().first()
    }
}
